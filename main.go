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
	a := app.NewWithID("com.roffe.t7l")
	vars := kwp2000.NewVarDefinitionList()
	sm := sink.NewManager()
	go dashboard.StartWebserver(a.Metadata().Release, sm, vars)
	mw := windows.NewMainWindow(a, sm, vars)
	mw.SetMaster()
	mw.Resize(fyne.NewSize(1400, 800))
	mw.SetContent(mw.Layout())
	mw.ShowAndRun()
}
