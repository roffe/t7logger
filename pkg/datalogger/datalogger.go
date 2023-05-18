package datalogger

import (
	"fmt"

	"fyne.io/fyne/v2/data/binding"
	"github.com/roffe/gocan"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
)

const ISO8601 = "2006-01-02T15:04:05.999-0700"

type DataClient interface {
	Start() error
	Close()
}

type Config struct {
	ECU                   string
	Dev                   gocan.Adapter
	Variables             []*kwp2000.VarDefinition
	Freq                  int
	OnMessage             func(string)
	CaptureCounter        binding.Int
	ErrorCounter          binding.Int
	ErrorPerSecondCounter binding.Int
	Sink                  *sink.Manager
}

func New(cfg Config) (DataClient, error) {
	switch cfg.ECU {
	case "T7":
		return NewT7(cfg)
	default:
		return nil, fmt.Errorf("%s not supported yet", cfg.ECU)
	}
}
