package kwp2000

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/roffe/gocan"
)

type Client struct {
	c *gocan.Client
	//canID             uint32
	//recvID            []uint32

	responseID        uint32
	defaultTimeout    time.Duration
	gotSequrityAccess bool
}

type KWPRequest struct {
}

type KWPReply struct {
}

func New(c *gocan.Client /*canID uint32, recvID ...uint32*/) *Client {
	return &Client{
		c: c,
		//canID:          canID,
		//recvID:         recvID,

		defaultTimeout: 250 * time.Millisecond,
	}
}

func (t *Client) StartSession(ctx context.Context, id, responseID uint32) error {
	payload := []byte{0x3F, START_COMMUNICATION, 0x00, 0x11, byte(REQ_MSG_ID >> 8), byte(REQ_MSG_ID), 0x00, 0x00}
	frame := gocan.NewFrame(id, payload, gocan.ResponseRequired)
	resp, err := t.c.SendAndPoll(ctx, frame, t.defaultTimeout, responseID)
	if err != nil {
		return err
	}

	data := resp.Data()
	if data[3] != START_COMMUNICATION|0x40 {
		return TranslateErrorCode(GENERAL_REJECT)
	}

	t.responseID = uint32(data[6])<<8 | uint32(data[7])

	//log.Printf("ECU reports responseID: 0x%03X", t.responseID)
	//log.Println(resp.String())
	return nil
}

func (t *Client) StopSession(ctx context.Context) error {
	payload := []byte{0x40, 0xA1, 0x02, STOP_COMMUNICATION, 0x00}
	frame := gocan.NewFrame(REQ_MSG_ID, payload, gocan.ResponseRequired)
	return t.c.Send(frame)
}

func (t *Client) StartRoutineByIdentifier(ctx context.Context, id byte) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x03, START_ROUTINE_BY_LOCAL_IDENTIFIER, id, 0x10}, gocan.ResponseRequired)
	log.Println(frame.String())
	resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}

	log.Println(resp.String())

	d := resp.Data()
	if d[3] == 0x7F {
		return nil, fmt.Errorf("StartRoutineByIdentifier: %w", TranslateErrorCode(d[5]))
	}

	return d, nil
}

func (t *Client) StopRoutineByIdentifier(ctx context.Context, id byte) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, STOP_ROUTINE_BY_LOCAL_IDENTIFIER, id}, gocan.ResponseRequired)
	log.Println(frame.String())
	resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}
	log.Println(resp.String())
	d := resp.Data()
	if d[3] == 0x7F {
		return nil, fmt.Errorf("StopRoutineByIdentifier: %w", TranslateErrorCode(d[5]))
	}
	return d, nil
}

func (t *Client) RequestRoutineResultsByLocalIdentifier(ctx context.Context, id byte) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, REQUEST_ROUTINE_RESULTS_BY_LOCAL_IDENTIFIER, id}, gocan.ResponseRequired)
	log.Println(frame.String())
	resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}

	log.Println(resp.String())

	d := resp.Data()
	if d[3] == 0x7F {
		return nil, fmt.Errorf("RequestRoutineResultsByLocalIdentifier: %w", TranslateErrorCode(d[5]))
	}
	return d, nil
}

func (t *Client) ReadDataByLocalIdentifier2(ctx context.Context, id byte) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, 0x50, id, 0x00, 0x00, 0x00}, gocan.ResponseRequired)
	log.Println(frame.String())
	resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}

	out := bytes.NewBuffer(nil)

	log.Println(resp.String())

	d := resp.Data()
	if d[3] == 0x7F {
		return nil, fmt.Errorf("ReadDataByLocalIdentifier2: %w", TranslateErrorCode(d[5]))
	}

	dataLenLeft := d[2] - 2
	//log.Println(resp.String())
	//log.Printf("data len left: %d", dataLenLeft)

	var thisRead byte
	if dataLenLeft > 3 {
		thisRead = 3
	} else {
		thisRead = dataLenLeft
	}

	out.Write(d[5 : 5+thisRead])
	dataLenLeft -= thisRead

	//log.Printf("data len left: %d", dataLenLeft)
	//log.Println(resp.String())

	currentChunkNumber := d[0] & 0x3F

	for currentChunkNumber != 0 {
		//log.Printf("current chunk %02X", currentChunkNumber)
		frame := gocan.NewFrame(RESP_CHUNK_CONF_ID, []byte{0x40, 0xA1, 0x3F, d[0] &^ 0x40, 0x00, 0x00, 0x00, 0x00}, gocan.ResponseRequired)
		//log.Println(frame.String())
		resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
		if err != nil {
			return nil, err
		}
		d = resp.Data()

		toRead := uint8(math.Min(6, float64(dataLenLeft)))
		//log.Println("bytes to read", toRead)
		out.Write(d[2 : 2+toRead])
		dataLenLeft -= toRead
		//log.Printf("data len left: %d", dataLenLeft)
		currentChunkNumber = d[0] & 0x3F
		//log.Printf("next chunk %02X", currentChunkNumber)
	}

	return out.Bytes(), nil
}

func (t *Client) ReadDataByLocalIdentifier(ctx context.Context, id byte) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, READ_DATA_BY_LOCAL_IDENTIFIER, id, 0x00, 0x00, 0x00}, gocan.ResponseRequired)
	resp, err := t.c.SendAndPoll(ctx, frame, 50*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}
	out := bytes.NewBuffer(nil)

	d := resp.Data()
	if d[3] == 0x7F {
		return nil, fmt.Errorf("ReadDataByLocalIdentifier: %w", TranslateErrorCode(d[5]))
	}

	dataLenLeft := d[2] - 2
	//log.Println(resp.String())
	//log.Printf("data len left: %d", dataLenLeft)

	var thisRead byte
	if dataLenLeft > 3 {
		thisRead = 3
	} else {
		thisRead = dataLenLeft
	}

	out.Write(d[5 : 5+thisRead])
	dataLenLeft -= thisRead

	//log.Printf("data len left: %d", dataLenLeft)
	//log.Println(resp.String())

	currentChunkNumber := d[0] & 0x3F

	for currentChunkNumber != 0 {
		//log.Printf("current chunk %02X", currentChunkNumber)
		frame := gocan.NewFrame(RESP_CHUNK_CONF_ID, []byte{0x40, 0xA1, 0x3F, d[0] &^ 0x40, 0x00, 0x00, 0x00, 0x00}, gocan.ResponseRequired)
		//log.Println(frame.String())
		resp, err := t.c.SendAndPoll(ctx, frame, 450*time.Millisecond, t.responseID)
		if err != nil {
			return nil, err
		}
		d = resp.Data()

		toRead := uint8(math.Min(6, float64(dataLenLeft)))
		//log.Println("bytes to read", toRead)
		out.Write(d[2 : 2+toRead])
		dataLenLeft -= toRead
		//log.Printf("data len left: %d", dataLenLeft)
		currentChunkNumber = d[0] & 0x3F
		//log.Printf("next chunk %02X", currentChunkNumber)
	}

	return out.Bytes(), nil
}

func (t *Client) TransferData(ctx context.Context) ([]byte, error) {
	frame := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, TRANSFER_DATA, 0x00}, gocan.ResponseRequired)
	log.Println(frame.String())
	resp, err := t.c.SendAndPoll(ctx, frame, 250*time.Millisecond, t.responseID)
	if err != nil {
		return nil, err
	}

	log.Println(resp.String())

	d := resp.Data()
	if d[3] == 0x7F {
		return nil, TranslateErrorCode(d[5])
	}
	return d, nil
}

func (t *Client) DynamicallyDefineLocalIdRequest(ctx context.Context, id int, v *VarDefinition) error {
	buff := bytes.NewBuffer(nil)
	buff.WriteByte(0xF0)
	switch v.Method {
	case VAR_METHOD_ADDRESS:
		buff.Write([]byte{0x03, byte(id), uint8(v.Length), byte(v.Value >> 16), byte(v.Value >> 8), byte(v.Value)})
	case VAR_METHOD_LOCID:
		buff.Write([]byte{0x01, byte(id), 0x00, byte(v.Value), 0x00})
	case VAR_METHOD_SYMBOL:
		buff.Write([]byte{0x03, byte(id), 0x00, 0x80, byte(v.Value >> 8), byte(v.Value)})
	}

	message := append([]byte{byte(buff.Len()), DYNAMICALLY_DEFINE_LOCAL_IDENTIFIER}, buff.Bytes()...)
	for _, msg := range t.splitRequest(message) {
		if msg.Type().Type == 1 {
			if err := t.c.Send(msg); err != nil {
				return err
			}
		} else {
			resp, err := t.c.SendAndPoll(ctx, msg, t.defaultTimeout, REQ_CHUNK_CONF_ID)
			if err != nil {
				return err
			}
			if err := TranslateErrorCode(resp.Data()[3+2]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Client) RequestSecurityAccess(ctx context.Context, force bool) (bool, error) {
	if t.gotSequrityAccess && !force {
		return true, nil
	}
	for i := 0; i <= 4; i++ {
		ok, err := t.letMeIn(ctx, i)
		if err != nil {
			log.Printf("/!\\ Failed to obtain security access: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if ok {
			t.gotSequrityAccess = true
			return true, nil
		}
	}

	return false, errors.New("RequestSecurityAccess: access was not granted")
}

func (t *Client) letMeIn(ctx context.Context, method int) (bool, error) {
	msg := []byte{0x40, 0xA1, 0x02, 0x27, 0x05, 0x00, 0x00, 0x00}
	msgReply := []byte{0x40, 0xA1, 0x04, 0x27, 0x06, 0x00, 0x00, 0x00}

	f, err := t.c.SendAndPoll(ctx, gocan.NewFrame(REQ_MSG_ID, msg, gocan.ResponseRequired), t.defaultTimeout, t.responseID)
	if err != nil {
		return false, fmt.Errorf("request seed: %v", err)

	}
	d := f.Data()
	t.Ack(d[0], gocan.ResponseRequired)

	s := int(d[5])<<8 | int(d[6])
	k := calcen(s, method)

	msgReply[5] = byte(int(k) >> 8 & int(0xFF))
	msgReply[6] = byte(k) & 0xFF

	f2, err := t.c.SendAndPoll(ctx, gocan.NewFrame(REQ_MSG_ID, msgReply, gocan.ResponseRequired), t.defaultTimeout, t.responseID)
	if err != nil {
		return false, fmt.Errorf("send seed: %v", err)

	}
	d2 := f2.Data()
	t.Ack(d2[0], gocan.ResponseRequired)
	if d2[3] == 0x67 && d2[5] == 0x34 {
		return true, nil
	} else {
		log.Println(f2.String())
		return false, errors.New("invalid response")
	}
}

// 266h Send acknowledgement, has 0x3F on 3rd!
func (t *Client) Ack(val byte, typ gocan.CANFrameType) error {
	ack := []byte{0x40, 0xA1, 0x3F, val & 0xBF, 0x00, 0x00, 0x00, 0x00}
	return t.c.Send(gocan.NewFrame(0x266, ack, typ))
}

func calcen(seed int, method int) int {
	key := seed << 2
	key &= 0xFFFF
	switch method {
	case 0:
		key ^= 0x8142
		key -= 0x2356
	case 1:
		key ^= 0x4081
		key -= 0x1F6F
	case 2:
		key ^= 0x3DC
		key -= 0x2356
	case 3:
		key ^= 0x3D7
		key -= 0x2356
	case 4:
		key ^= 0x409
		key -= 0x2356
	}
	key &= 0xFFFF
	return key
}

func (t *Client) SendRequest(req *KWPRequest) (*KWPReply, error) {
	return nil, nil
}

func (t *Client) splitRequest(payload []byte) []gocan.CANFrame {
	msgCount := (len(payload) + 6 - 1) / 6

	var results []gocan.CANFrame

	for i := 0; i < msgCount; i++ {
		msgData := make([]byte, 8)

		flag := 0

		if i == 0 {
			flag |= 0x40 // this is the first data chunk
		}

		if i != msgCount-1 {
			flag |= 0x80 // we want confirmation for every chunk except the last one
		}
		msgData[0] = (byte)(flag | ((msgCount - i - 1) & 0x3F)) // & 0x3F is not necessary, only to show that this field is 6-bit wide
		msgData[1] = 0xA1

		start := 6 * i
		var count int
		if len(payload)-start < 6 {
			count = len(payload) - start
		} else {
			count = 6
		}

		copy(msgData[2:], payload[start:start+count])
		for j := 0; j < count; j++ {
			msgData[2+j] = payload[start+j]
		}

		if flag&0x80 == 0x80 {
			results = append(results, gocan.NewFrame(REQ_MSG_ID, msgData, gocan.ResponseRequired))
		} else {
			results = append(results, gocan.NewFrame(REQ_MSG_ID, msgData, gocan.Outgoing))
		}

	}

	return results
}

func (t *Client) recvData(ctx context.Context, length int) ([]byte, error) {
	var receivedBytes, payloadLeft int
	out := bytes.NewBuffer([]byte{})

	sub := t.c.Subscribe(ctx, t.responseID)
	startTransfer := gocan.NewFrame(REQ_MSG_ID, []byte{0x40, 0xA1, 0x02, 0x21, 0xF0, 0x00, 0x00, 0x00}, gocan.ResponseRequired)
	if err := t.c.Send(startTransfer); err != nil {
		return nil, err
	}

outer:
	for receivedBytes < length {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(t.defaultTimeout * 4):
			return nil, fmt.Errorf("timeout")

		case f := <-sub:
			d := f.Data()
			if d[0]&0x40 == 0x40 {
				payloadLeft = int(d[2]) - 2 // subtract two non-payload bytes

				if payloadLeft > 0 && receivedBytes < length {
					out.WriteByte(d[5])
					receivedBytes++
					payloadLeft--
				}
				if payloadLeft > 0 && receivedBytes < length {
					out.WriteByte(d[6])
					receivedBytes++
					payloadLeft--
				}
				if payloadLeft > 0 && receivedBytes < length {
					out.WriteByte(d[7])
					receivedBytes++
					payloadLeft--
				}
			} else {
				for i := 0; i < 6; i++ {
					if receivedBytes < length {
						out.WriteByte(d[2+i])
						receivedBytes++
						payloadLeft--
						if payloadLeft == 0 {
							break
						}
					}
				}
			}
			if d[0] == 0x80 || d[0] == 0xC0 {
				t.Ack(d[0], gocan.Outgoing)
				break outer
			} else {
				t.Ack(d[0], gocan.ResponseRequired)
			}
		}
	}
	return out.Bytes(), nil
}
