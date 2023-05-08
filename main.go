package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	//xlayout "fyne.io/x/fyne/layout"

	"github.com/roffe/t7logger/pkg/dashboard"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
	"github.com/roffe/t7logger/pkg/windows"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
}

func main() {
	vars := kwp2000.NewVarDefinitionList()
	sm := sink.NewManager()
	//sub := sinkManager.NewSubscriber(func(msg string) {
	//	fmt.Println("msg:", msg)
	//})
	//defer sub.Close()
	a := app.NewWithID("com.roffe.t7l")

	go dashboard.StartWebserver(a.Metadata().Release, sm, vars)
	mw := windows.NewMainWindow(a, sm, vars)
	mw.SetMaster()
	mw.Resize(fyne.NewSize(1400, 800))
	mw.SetContent(mw.Layout())
	mw.ShowAndRun()
}
