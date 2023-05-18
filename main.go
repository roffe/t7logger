package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	//xlayout "fyne.io/x/fyne/layout"

	"github.com/roffe/t7logger/dashboard"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
	"github.com/roffe/t7logger/pkg/windows"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
}

func main() {
	ready := make(chan struct{})
	a := app.NewWithID("com.roffe.trl")
	vars := kwp2000.NewVarDefinitionList()
	sm := sink.NewManager()
	mw := windows.NewMainWindow(a, sm, vars)
	go dashboard.Start(mw.Log, a.Metadata().Release, sm, vars, ready)
	mw.SetMaster()
	mw.Resize(fyne.NewSize(1400, 800))
	mw.SetContent(mw.Layout())
	close(ready)
	mw.ShowAndRun()
}
