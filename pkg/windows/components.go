package windows

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	xwidget "fyne.io/x/fyne/widget"
	"github.com/roffe/t7logger/pkg/datalogger"
	"github.com/roffe/t7logger/pkg/widgets"
)

func (mw *MainWindow) newLogBtn() {
	mw.logBtn = widget.NewButtonWithIcon("Start logging", theme.DownloadIcon(), func() {
		if mw.loggingRunning {
			if mw.dlc != nil {
				mw.dlc.Close()
			}
			return
		}
		if !mw.loggingRunning {
			device, err := mw.canSettings.GetAdapter(mw.Log)
			if err != nil {
				dialog.ShowError(err, mw)
				return
			}

			mw.dlc, err = datalogger.New(datalogger.Config{
				ECU:                   mw.ecuSelect.Selected,
				Dev:                   device,
				Variables:             mw.vars.Get(),
				Freq:                  int(mw.freqSlider.Value),
				OnMessage:             mw.Log,
				CaptureCounter:        mw.captureCounter,
				ErrorCounter:          mw.errorCounter,
				ErrorPerSecondCounter: mw.errorPerSecondCounter,
				Sink:                  mw.sinkManager,
			})
			if err != nil {
				dialog.ShowError(err, mw)
				return
			}
			go func() {
				mw.loggingRunning = true
				mw.logBtn.SetText("Stop logging")
				mw.disableBtns()
				defer mw.enableBtns()
				mw.mockBtn.Disable()
				defer mw.mockBtn.Enable()
				mw.progressBar.Start()
				if err := mw.dlc.Start(); err != nil {
					dialog.ShowError(err, mw)
				}
				mw.progressBar.Stop()
				mw.loggingRunning = false
				mw.dlc = nil
				mw.logBtn.SetText("Start logging")
			}()
		}
	})
}

func (mw *MainWindow) newOutputList() {
	mw.output = widget.NewListWithData(
		mw.outputData,
		func() fyne.CanvasObject {
			return &widget.Label{
				Alignment: fyne.TextAlignLeading,
				Wrapping:  fyne.TextWrapBreak,
				TextStyle: fyne.TextStyle{Monospace: true},
			}
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			i := item.(binding.String)
			txt, err := i.Get()
			if err != nil {
				mw.Log(err.Error())
				return
			}
			obj.(*widget.Label).SetText(txt)
		},
	)

	mw.symbolConfigList = widget.NewList(
		func() int {
			return mw.vars.Len()
		},
		func() fyne.CanvasObject {
			return widgets.NewVarDefinitionWidget(mw.symbolConfigList, mw.vars)
		},
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			coo := co.(*widgets.VarDefinitionWidget)
			coo.Update(lii, mw.vars.GetPos(lii))
		},
	)
}

func (mw *MainWindow) newSymbolnameTypeahead() {
	mw.symbolLookup = xwidget.NewCompletionEntry([]string{})
	mw.symbolLookup.PlaceHolder = "Type to search for symbols"
	// When the use typed text, complete the list.
	mw.symbolLookup.OnChanged = func(s string) {
		// completion start for text length >= 3
		if len(s) < 3 {
			mw.symbolLookup.HideCompletion()
			return
		}
		// Get the list of possible completion
		var results []string
		for _, sym := range mw.symbolMap {
			if strings.Contains(strings.ToLower(sym.Name), strings.ToLower(s)) {
				results = append(results, sym.Name)
			}
		}
		// no results
		if len(results) == 0 {
			mw.symbolLookup.HideCompletion()
			return
		}
		sort.Slice(results, func(i, j int) bool { return strings.ToLower(results[i]) < strings.ToLower(results[j]) })

		// then show them
		mw.symbolLookup.SetOptions(results)
		mw.symbolLookup.ShowCompletion()
	}
}
