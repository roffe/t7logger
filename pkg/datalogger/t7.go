package datalogger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/roffe/gocan"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
)

type T7Client struct {
	quitChan chan struct{}
	out      strings.Builder
	Config
}

func NewT7(cfg Config) (*T7Client, error) {
	return &T7Client{
		quitChan: make(chan struct{}, 2),
		Config:   cfg,
	}, nil
}

func (c *T7Client) Close() {
	c.quitChan <- struct{}{}
	time.Sleep(200 * time.Millisecond)
}

func (c *T7Client) Start() error {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		if err := os.Mkdir("logs", 0755); err != nil {
			if err != os.ErrExist {
				return fmt.Errorf("failed to create logs dir: %w", err)
			}
		}
	}
	filename := fmt.Sprintf("logs/log-%s.t7l", time.Now().Format("2006-01-02-15-04-05"))
	c.OnMessage(fmt.Sprintf("Logging to %s", filename))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	ctx := context.Background()

	//inHandler := func(msg gocan.CANFrame) {
	//	log.Println(msg.String())
	//}
	//
	//outHandler := func(msg gocan.CANFrame) {
	//	log.Println(msg.String())
	//}

	cl, err := gocan.NewWithOpts(
		ctx,
		c.Dev,
		//gocan.OptOnIncoming(inHandler),
		//gocan.OptOnOutgoing(outHandler),
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	kwp := kwp2000.New(cl)

	count := 0
	errCount := 0
	c.ErrorCounter.Set(errCount)

	errPerSecond := 0
	c.ErrorPerSecondCounter.Set(errPerSecond)

	cps := 0
	retries := 0

	err = retry.Do(func() error {
		if err := kwp.StartSession(ctx, kwp2000.INIT_MSG_ID, kwp2000.INIT_RESP_ID); err != nil {
			if retries == 0 {
				return retry.Unrecoverable(err)
			}
			return err
		}
		defer func() {
			kwp.StopSession(ctx)
			time.Sleep(50 * time.Millisecond)
		}()

		c.OnMessage("Connected to ECU")

		for i, v := range c.Variables {
			//c.onMessage(fmt.Sprintf("%d %s %s %d %X", i, v.Name, v.Method, v.Value, v.Type))
			if err := kwp.DynamicallyDefineLocalIdRequest(ctx, i, v); err != nil {
				return fmt.Errorf("DynamicallyDefineLocalIdRequest: %w", err)
			}
			time.Sleep(5 * time.Millisecond)
		}

		secondTicker := time.NewTicker(time.Second)
		defer secondTicker.Stop()

		t := time.NewTicker(time.Second / time.Duration(c.Freq))
		defer t.Stop()

		c.OnMessage(fmt.Sprintf("Live logging at %d fps", c.Freq))
		for {
			select {
			case <-c.quitChan:
				c.OnMessage("Stop logging...")
				return nil
			case <-secondTicker.C: // every time the ticker ticks
				log.Println("cps:", cps)
				cps = 0
				c.ErrorPerSecondCounter.Set(errPerSecond)
				if errPerSecond > 10 {
					errPerSecond = 0
					return fmt.Errorf("too many errors, restarting logging")
				}
				errPerSecond = 0
			case <-t.C:
				data, err := kwp.ReadDataByLocalIdentifier(ctx, 0xF0)
				if err != nil {
					errCount++
					errPerSecond++
					c.ErrorCounter.Set(errCount)
					c.OnMessage(fmt.Sprintf("Failed to read data: %v", err))
					continue
				}
				r := bytes.NewReader(data)
				for _, va := range c.Variables {
					if err := va.Read(r); err != nil {
						c.OnMessage(fmt.Sprintf("Failed to read %s: %v", va.Name, err))
						break
					}
				}
				if r.Len() > 0 {
					left := r.Len()
					leftovers := make([]byte, r.Len())
					n, err := r.Read(leftovers)
					if err != nil {
						c.OnMessage(fmt.Sprintf("Failed to read leftovers: %v", err))
					}
					c.OnMessage(fmt.Sprintf("Leftovers %d: %X", left, leftovers[:n]))
				}
				c.produceLogLine(file, c.Variables)
				count++
				cps++
				c.CaptureCounter.Set(count)
			}
		}
	},
		retry.DelayType(retry.FixedDelay),
		retry.Delay(500*time.Millisecond),
		retry.Attempts(10),
		retry.OnRetry(func(n uint, err error) {
			retries++
			c.OnMessage(fmt.Sprintf("Retry %d: %v", n, err))
		}),
	)
	return err
}

func (c *T7Client) produceLogLine(file io.Writer, vars []*kwp2000.VarDefinition) {
	c.out.WriteString("|")
	var ms []string
	for _, va := range vars {
		c.out.WriteString(va.T7L() + "|")
		ms = append(ms, va.Tuple())
	}
	fmt.Fprintln(file, time.Now().Format("02-01-2006 15:04:05.999")+c.out.String()+"IMPORTANTLINE=0|")
	c.Sink.Push(&sink.Message{
		Data: []byte(time.Now().Format(ISO8601) + "|" + strings.Join(ms, ",")),
	})
	c.out.Reset()
}
