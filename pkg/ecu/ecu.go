package ecu

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/roffe/gocan"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/symbol"
)

func GetSymbols(ctx context.Context, dev gocan.Adapter, cb func(string)) ([]*symbol.Symbol, error) {
	/*
		logger := func(s string) {
			log.Println(s)
		}
		dev, err := adapter.New(
			//"STN2120",
			"CANUSB",
			&gocan.AdapterConfig{
				Port:         "COM7",
				PortBaudrate: 2000000,
				CANRate:      500,
				CANFilter:    []uint32{0x238, 0x258, 0x270},
				OnMessage:    logger,
				OnError: func(err error) {
					logger(err.Error())
				},
			},
		)
		if err != nil {
			return nil, err
		}
	*/

	cl, err := gocan.New(context.TODO(), dev)
	if err != nil {
		return nil, err
	}
	defer cl.Close()

	k := kwp2000.New(cl)
	if err := k.StartSession(ctx, kwp2000.INIT_MSG_ID, kwp2000.INIT_RESP_ID); err != nil {
		return nil, err
	}
	defer k.StopSession(ctx)

	cb("Connected to ECU")

	granted, err := k.RequestSecurityAccess(ctx, false)
	if err != nil {
		return nil, err
	}

	if !granted {
		return nil, errors.New("security access not granted")
	}

	if err := k.StartRoutineByIdentifier(ctx, 0x50); err != nil {
		return nil, err
	}

	cb("Fetching Symbol Table")
	start := time.Now()
	symTable, err := k.TransferData(ctx)
	if err != nil {
		return nil, err
	}
	// log.Println("time to read symbol table", time.Since(start))
	// cb(fmt.Sprintf("Took %s to load Symbol Table", time.Since(start)))

	if err := k.RequestTransferExit(ctx); err != nil {
		return nil, err
	}

	sym_count := 0
	var symbols []*symbol.Symbol
	buff := bytes.NewReader(symTable)
	for buff.Len() > 0 {
		var addr uint32
		if err := binary.Read(buff, binary.BigEndian, &addr); err != nil {
			return nil, err
		}
		var length uint16
		if err := binary.Read(buff, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		var symType uint8
		if err := binary.Read(buff, binary.BigEndian, &symType); err != nil {
			return nil, err
		}
		symbols = append(symbols, &symbol.Symbol{
			Name:    fmt.Sprintf("Symbol-%d", sym_count),
			Number:  sym_count,
			Address: addr,
			Length:  length,
			Type:    symType,
		})
		sym_count++
	}

	//ff, err := os.Create("symdebug.txt")
	//if err != nil {
	//	return nil, err
	//}
	//defer ff.Close()

	cb("Fetching Symbol Names")

	compressedSymbolNameTable, err := k.ReadFlash(ctx, int(symbols[0].Address), int(symbols[0].Length))
	if err != nil {
		return nil, err
	}
	// log.Println("time to read symbol name table", time.Since(start))
	// log.Println("size symbolnames", len(compressedSymbolNameTable))

	symbolNames, err := symbol.ExpandCompressedSymbolNames(compressedSymbolNameTable)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(symbolNames)-1; i++ {
		symbols[i].Name = strings.TrimSpace(symbolNames[i])
		symbols[i].Unit = symbol.GetUnit(symbols[i].Name)
		symbols[i].Correctionfactor = symbol.GetCorrectionfactor(symbols[i].Name)
	}
	cb(fmt.Sprintf("Loaded %d symbols from ECU in %s", sym_count, time.Since(start).Round(time.Millisecond).String()))

	return symbols, nil
}
