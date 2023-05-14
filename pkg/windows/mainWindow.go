package windows

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	xwidget "fyne.io/x/fyne/widget"
	"github.com/roffe/t7logger/pkg/datalogger"
	"github.com/roffe/t7logger/pkg/ecu"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
	"github.com/roffe/t7logger/pkg/symbol"
	"github.com/roffe/t7logger/pkg/widgets"
	sdialog "github.com/sqweek/dialog"
	"golang.org/x/net/context"
)

const (
	prefsLastConfig = "lastConfig"
)

type MainWindow struct {
	fyne.Window
	app fyne.App

	symbolMap map[string]*kwp2000.VarDefinition

	symbolLookup     *xwidget.CompletionEntry
	symbolConfigList *widget.List

	output     *widget.List
	outputData binding.StringList

	canSettings *widgets.CanSettingsWidget

	addSymbolBtn       *widget.Button
	logBtn             *widget.Button
	mockBtn            *widget.Button
	loadSymbolsEcuBtn  *widget.Button
	loadSymbolsFileBtn *widget.Button

	loadConfigBtn  *widget.Button
	saveConfigBtn  *widget.Button
	syncSymbolsBtn *widget.Button

	captureCounter        binding.Int
	errorCounter          binding.Int
	errorPerSecondCounter binding.Int
	freqValue             binding.Float
	progressBar           *widget.ProgressBarInfinite

	freqSlider *widget.Slider

	sinkManager *sink.Manager

	loggingRunning bool
	mockRunning    bool

	dlc  *datalogger.Client
	vars *kwp2000.VarDefinitionList

	debuglog *os.File
}

func (mw *MainWindow) disableBtns() {
	mw.loadConfigBtn.Disable()
	mw.saveConfigBtn.Disable()
	mw.syncSymbolsBtn.Disable()
	mw.loadSymbolsFileBtn.Disable()
	mw.loadSymbolsEcuBtn.Disable()
	mw.logBtn.Disable()
	mw.mockBtn.Disable()
	mw.canSettings.Disable()
}

func (mw *MainWindow) enableBtns() {
	mw.loadConfigBtn.Enable()
	mw.saveConfigBtn.Enable()
	mw.syncSymbolsBtn.Enable()
	mw.loadSymbolsFileBtn.Enable()
	mw.loadSymbolsEcuBtn.Enable()
	mw.logBtn.Enable()
	mw.mockBtn.Enable()
	mw.canSettings.Enable()
}

func NewMainWindow(a fyne.App, singMgr *sink.Manager, vars *kwp2000.VarDefinitionList) *MainWindow {
	mw := &MainWindow{
		Window:                a.NewWindow("Trionic7 Logger - No file loaded"),
		app:                   a,
		symbolMap:             make(map[string]*kwp2000.VarDefinition),
		outputData:            binding.NewStringList(),
		canSettings:           widgets.NewCanSettingsWidget(a),
		captureCounter:        binding.NewInt(),
		errorCounter:          binding.NewInt(),
		errorPerSecondCounter: binding.NewInt(),
		freqValue:             binding.NewFloat(),
		progressBar:           widget.NewProgressBarInfinite(),
		sinkManager:           singMgr,
		vars:                  vars,
	}

	f, err := os.OpenFile("debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		mw.debuglog = f
	}

	mw.Window.SetCloseIntercept(func() {
		if mw.debuglog != nil {
			mw.debuglog.Sync()
			mw.debuglog.Close()
		}
		mw.Close()
	})

	mw.addSymbolBtn = widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		defer mw.symbolConfigList.Refresh()
		s, ok := mw.symbolMap[mw.symbolLookup.Text]
		if !ok {
			mw.vars.Add(&kwp2000.VarDefinition{
				Name: mw.symbolLookup.Text,
			})
			return
		}
		mw.vars.Add(s)
		//log.Printf("Name: %s, Method: %d, Value: %d, Type: %X", s.Name, s.Method, s.Value, s.Type)
	})

	mw.loadSymbolsFileBtn = widget.NewButtonWithIcon("Load from binary", theme.FileIcon(), func() {
		filename, err := sdialog.File().Filter("Binary file", "bin").Load()
		if err != nil {
			if err.Error() == "Cancelled" {
				return
			}
			dialog.ShowError(err, mw)
		}
		if err := mw.loadSymbolsFromFile(filename); err != nil {
			dialog.ShowError(err, mw)
			return
		}
	})

	mw.loadSymbolsEcuBtn = widget.NewButtonWithIcon("Load from ECU", theme.DownloadIcon(), func() {
		mw.progressBar.Start()
		mw.disableBtns()
		defer mw.enableBtns()
		defer mw.progressBar.Stop()
		if err := mw.loadSymbolsFromECU(); err != nil {
			dialog.ShowError(err, mw)
			return
		}
	})

	mw.loadConfigBtn = widget.NewButtonWithIcon("Load config", theme.FileIcon(), func() {
		filename, err := sdialog.File().Filter("*.json", "json").Load()
		if err != nil {
			if err.Error() == "Cancelled" {
				return
			}
			dialog.ShowError(err, mw)
		}
		if err := mw.LoadConfig(filename); err != nil {
			dialog.ShowError(err, mw)
			return
		}
		mw.symbolConfigList.Refresh()
	})

	mw.saveConfigBtn = widget.NewButtonWithIcon("Save config", theme.DocumentSaveIcon(), func() {
		filename, err := sdialog.File().Filter("*.json", "json").Save()
		if err != nil {
			if err.Error() == "Cancelled" {
				return
			}
			dialog.ShowError(err, mw)
		}
		if err := mw.SaveConfig(filename); err != nil {
			dialog.ShowError(err, mw)
			return

		}
	})

	mw.syncSymbolsBtn = widget.NewButtonWithIcon("Sync symbols", theme.ViewRefreshIcon(), func() {
		for i, v := range mw.vars.Get() {
			for k, vv := range mw.symbolMap {
				if strings.EqualFold(k, v.Name) {
					mw.vars.UpdatePos(i, vv)
					break
				}
			}
		}
		mw.symbolConfigList.Refresh()
	})

	mw.progressBar.Stop()

	mw.freqSlider = widget.NewSliderWithData(1, 120, mw.freqValue)
	mw.freqSlider.SetValue(25)

	mw.output = mw.newOutputList()
	mw.symbolLookup = mw.newSymbolnameTypeahead()
	mw.logBtn = mw.newLogBtn()
	mw.mockBtn = mw.newMockBtn()

	if filename := mw.app.Preferences().String(prefsLastConfig); filename != "" {
		mw.LoadConfig(filename)
	}

	return mw
}

func (mw *MainWindow) setTitle(str string) {
	mw.SetTitle("Trionic7 Logger - " + str)
}

func (mw *MainWindow) Layout() fyne.CanvasObject {
	capturedCounter := widget.NewLabel("")
	capturedCounter.Alignment = fyne.TextAlignLeading

	errorCounter := widget.NewLabel("")
	errorCounter.Alignment = fyne.TextAlignLeading

	mw.captureCounter.AddListener(binding.NewDataListener(func() {
		if val, err := mw.captureCounter.Get(); err == nil {
			capturedCounter.SetText(fmt.Sprintf("Cap: %d", val))
		}
	}))

	mw.errorCounter.AddListener(binding.NewDataListener(func() {
		if val, err := mw.errorCounter.Get(); err == nil {
			errorCounter.SetText(fmt.Sprintf("Err: %d", val))
		}
	}))

	errorPerSecondCounter := widget.NewLabel("")
	errorPerSecondCounter.Alignment = fyne.TextAlignLeading

	mw.errorPerSecondCounter.AddListener(binding.NewDataListener(func() {
		if val, err := mw.errorPerSecondCounter.Get(); err == nil {
			errorPerSecondCounter.SetText(fmt.Sprintf("Err/s: %d", val))
		}
	}))

	freqValue := widget.NewLabel("")

	mw.freqValue.AddListener(binding.NewDataListener(func() {
		if val, err := mw.freqValue.Get(); err == nil {
			freqValue.SetText(fmt.Sprintf("Freq: %0.f", val))
		}
	}))

	left := container.NewBorder(
		container.NewVBox(
			container.NewBorder(
				nil,
				nil,
				widget.NewLabel("Symbol lookup"),
				container.NewHBox(
					mw.addSymbolBtn,
					mw.loadSymbolsFileBtn,
					mw.loadSymbolsEcuBtn,
				),
				mw.symbolLookup,
			),
			container.NewHBox(
				widgets.MinWidth(250, &widget.Label{
					Text:      "Name",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(90, &widget.Label{
					Text:      "Method",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(50, &widget.Label{
					Text:      "#",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(40, &widget.Label{
					Text:      "Type",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(80, &widget.Label{
					Text:      "Signed",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(50, &widget.Label{
					Text:      "Factor",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(130, &widget.Label{
					Text:      "Group",
					Alignment: fyne.TextAlignLeading,
				}),
				widgets.MinWidth(90, &widget.Label{
					Text:      "",
					Alignment: fyne.TextAlignLeading,
				}),
			),
		),
		container.NewVBox(
			container.NewGridWithColumns(4,
				mw.loadConfigBtn,
				mw.syncSymbolsBtn,
				mw.saveConfigBtn,
				widget.NewButtonWithIcon("Dashboard", theme.InfoIcon(), func() {
					mw.openBrowser("http://localhost:8080")
				}),
				//widget.NewButton("?", func() {
				//	b, err := json.MarshalIndent(mw.symbolMap, "", "  ")
				//	if err != nil {
				//		dialog.ShowError(err, mw)
				//		return
				//	}
				//	log.Println(string(b))
				//}),
			),
		),
		nil,
		nil,
		mw.symbolConfigList,
	)

	return &container.Split{
		Offset:     0.6,
		Horizontal: true,
		Leading:    left,
		Trailing: &container.Split{
			Offset:     0,
			Horizontal: false,
			Leading: container.NewVBox(
				mw.canSettings,
				mw.logBtn,
				mw.progressBar,
			),
			Trailing: &container.Split{
				Offset:     1,
				Horizontal: false,
				Leading:    mw.output,
				Trailing: container.NewVBox(
					mw.mockBtn,
					mw.freqSlider,
					container.NewGridWithColumns(4,
						capturedCounter,
						errorCounter,
						errorPerSecondCounter,
						freqValue,
					),
				),
			},
		},
	}

}

func (mw *MainWindow) loadSymbolsFromECU() error {
	device, err := mw.canSettings.GetAdapter(mw.writeOutput)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	symbols, err := ecu.GetSymbols(ctx, device, mw.writeOutput)
	if err != nil {
		return err
	}
	mw.loadSymbols(symbols)
	mw.setTitle("Symbols loaded from ECU " + time.Now().Format("2006-01-02 15:04:05.000"))
	return nil
}

func (mw *MainWindow) loadSymbolsFromFile(filename string) error {
	symbols, err := symbol.LoadSymbols(filename, mw.writeOutput)
	if err != nil {
		return fmt.Errorf("error loading symbols: %w", err)
	}
	mw.loadSymbols(symbols)
	mw.setTitle(filename)
	return nil
}

func (mw *MainWindow) loadSymbols(symbols []*symbol.Symbol) {
	newSymbolMap := make(map[string]*kwp2000.VarDefinition)
	for _, s := range symbols {
		def := &kwp2000.VarDefinition{
			Name:             s.Name,
			Method:           kwp2000.VAR_METHOD_SYMBOL,
			Value:            s.Number,
			Type:             s.Type,
			Length:           s.Length,
			Correctionfactor: s.Correctionfactor,
			Unit:             s.Unit,
		}
		newSymbolMap[s.Name] = def
	}
	mw.symbolMap = newSymbolMap
}

func (mw *MainWindow) openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		dialog.ShowError(err, mw)
	}
}

func (mw *MainWindow) writeOutput(s string) {
	if mw.debuglog != nil {
		mw.debuglog.WriteString(time.Now().Format("2006-01-02 15:04:05.000") + " " + s + "\n")
	}
	mw.outputData.Append(s)
	mw.output.ScrollToBottom()
}

func (mw *MainWindow) SaveConfig(filename string) error {
	b, err := json.Marshal(mw.vars.Get())
	if err != nil {
		return fmt.Errorf("failed to marshal config file: %w", err)
	}
	if err := os.WriteFile(filename, b, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	mw.app.Preferences().SetString(prefsLastConfig, filename)
	return nil
}

func (mw *MainWindow) LoadConfig(filename string) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg []*kwp2000.VarDefinition
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config file: %w", err)
	}
	mw.vars.Set(cfg)
	mw.app.Preferences().SetString(prefsLastConfig, filename)
	return nil
}
