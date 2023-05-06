package main

import (
	"log"
	"math/rand"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	//xlayout "fyne.io/x/fyne/layout"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
	"github.com/roffe/t7logger/pkg/windows"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
}

func main() {
	sm := sink.NewManager()

	vars := kwp2000.NewVarDefinitionList()

	go startWeb2(sm, vars)
	//sub := sinkManager.NewSubscriber(func(msg string) {
	//	fmt.Println("msg:", msg)
	//})
	//defer sub.Close()
	a := app.NewWithID("com.roffe.t7l")
	mw := windows.NewMainWindow(a, sm, vars)
	mw.SetMaster()
	mw.Resize(fyne.NewSize(1400, 800))
	mw.SetContent(mw.Layout())
	mw.ShowAndRun()
}

type SymbolDefinition struct {
	Name string
	ID   int
	Type string
}

/*
	var upgrader = websocket.Upgrader{
		//check origin will check the cross region source (note : please not using in production)
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			log.Println("origin:", origin)
			//return origin == "chrome-extension://cbcbkhdmedgianpaifchdaddpnmgnknn"
			return true
		},
	}

	func startWeb(sm *sink.Manager, vars *kwp2000.VarDefinitionList) {
		r := gin.Default()
		r.Use(cors.New(cors.Config{
			//AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST"},
			AllowHeaders:     []string{"Origin"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			AllowOriginFunc: func(origin string) bool {
				//return origin == "https://github.com"
				return true
			},
			//MaxAge: 12 * time.Hour,
		}))
		r.StaticFS("/public", http.Dir("./web"))
		r.GET("/ws", func(c *gin.Context) {
			//upgrade get request to websocket protocol
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				fmt.Println(err)
				return
			}

			var symbolList []SymbolDefinition
			for _, v := range vars.Get() {
				symbolList = append(symbolList, SymbolDefinition{
					Name: v.Name,
					ID:   v.Value,
					Type: returnVis(v.Visualization),
				})
			}

			defer ws.Close()

			for {
				//Read Message from client
				mt, message, err := ws.ReadMessage()
				if err != nil {
					fmt.Println(err)
					break
				}

				log.Printf("type %d, message %s", mt, message)

				if string(message) == "start_session" {
					log.Println("start_session")
					if err := ws.WriteJSON(gin.H{
						"type": "symbols",
						"data": symbolList,
					}); err != nil {
						fmt.Println(err)
						return
					}
					sub := sm.NewSubscriber(func(m *sink.Message) {
						if err := ws.WriteJSON(gin.H{
							"type": "metric",
							"data": string(m.Data),
						}); err != nil {
							fmt.Println(err)
							return
						}
					})
					defer sub.Close()
				}

				//If client message is ping will return pong
				//if string(message) == "ping" {
				//	message = []byte("pong")
				//}

				////Response message to client
				//err = ws.WriteMessage(mt, message)
				//if err != nil {
				//	fmt.Println(err)
				//	break
				//}

			}
		})
		r.Run(":8080") // listen and serve on 0.0.0.0:8080
	}
*/
func startWeb2(sm *sink.Manager, vars *kwp2000.VarDefinitionList) {
	router := gin.New()

	router.Use(cors.New(cors.Config{
		//AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			//return origin == "https://github.com"
			return true
		},
		//MaxAge: 12 * time.Hour,
	}))

	server := socketio.NewServer(nil)

	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("socket.io error:", e)
	})

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("closed", reason)
	})

	server.OnEvent("/", "end_session", func(s socketio.Conn) {
		s.Leave("metrics")
	})

	server.OnEvent("/", "start_session", func(s socketio.Conn, msg string) {
		s.Join("metrics")
	})

	server.OnEvent("/", "request_symbols", func(s socketio.Conn) {
		var symbolList []SymbolDefinition
		for _, v := range vars.Get() {
			//vis := "linegraph"
			symbolList = append(symbolList, SymbolDefinition{
				Name: v.Name,
				ID:   v.Value,
				Type: "linegraph",
			})
		}
		s.Emit("symbol_list", symbolList)
	})

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("socketio listen error: %s\n", err)
		}
	}()
	defer server.Close()

	sub := sm.NewSubscriber(func(msg *sink.Message) {
		if server.Count() > 0 {
			server.BroadcastToRoom("/", "metrics", "metrics", string(msg.Data))
		}
	})
	defer sub.Close()

	router.Use(static.Serve("/", static.LocalFile("./web", false)))
	router.GET("/socket.io/*any", gin.WrapH(server))
	router.POST("/socket.io/*any", gin.WrapH(server))

	if err := router.Run(":8080"); err != nil {
		log.Fatal("failed run app: ", err)
	}
}

var rr = rand.New(rand.NewSource(time.Now().UnixNano()))

func returnVis(t string) string {
	if t == "" {
		return "linegraph"
	}
	return t
}
