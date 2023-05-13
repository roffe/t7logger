package dashboard

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/roffe/t7logger/pkg/kwp2000"
	"github.com/roffe/t7logger/pkg/sink"
)

type SymbolDefinition struct {
	Name  string
	ID    int
	Type  string
	Unit  string
	Group string
}

//go:embed public
var public embed.FS

func StartWebserver(releaseMode bool, sm *sink.Manager, vars *kwp2000.VarDefinitionList, ready chan struct{}) {
	<-ready
	router := gin.Default()

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
		log.Println("socket.io connected:", s.ID())
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
			symbolList = append(symbolList, SymbolDefinition{
				Name:  v.Name,
				ID:    v.Value,
				Type:  returnVis(v.Visualization),
				Unit:  v.Unit,
				Group: v.Group,
			})
		}
		s.Emit("symbol_list", symbolList)
	})

	server.OnEvent("/", "list_logs", func(s socketio.Conn) {
		files, err := os.ReadDir("./logs")
		if err != nil {
			log.Println(err)
			return
		}

		var logfiles []LogFile
		for _, f := range files {
			logfiles = append(logfiles, LogFile{Name: f.Name()})
		}
		s.Emit("log_list", logfiles)
	})

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("socket.io listen error: %s\n", err)
		}
	}()
	defer server.Close()

	// Create a subscriber for the metrics topic that push messages to the socket.io server if there is clients connected
	sub := sm.NewSubscriber(func(msg *sink.Message) {
		if server.RoomLen("/", "metrics") > 0 {
			server.BroadcastToRoom("/", "metrics", "metrics", string(msg.Data))
		}
	})
	defer sub.Close()

	// Read files from disk if not running in release mode, else serve them from in-memory FS
	if !releaseMode {
		router.Use(static.Serve("/", static.LocalFile("./dashboard/public", false)))
	} else {
		subFS, err := fs.Sub(public, "public")
		if err != nil {
			log.Fatal(err)
		}
		router.Use(static.Serve("/", &HttpFileSystem{
			FileSystem: http.FS(subFS),
			root:       "/",
			indexes:    false,
		}))
	}

	// Handle socket.io routes
	router.GET("/socket.io/*any", gin.WrapH(server))
	router.POST("/socket.io/*any", gin.WrapH(server))

	// Start webserver
	if err := router.Run(":8080"); err != nil {
		log.Fatal("failed run app: ", err)
	}
}

type LogFile struct {
	Name string
}

func returnVis(t string) string {
	if t == "" {
		return "linegraph"
	}
	return t
}

type HttpFileSystem struct {
	http.FileSystem
	root    string
	indexes bool
}

func (l *HttpFileSystem) Exists(prefix string, filepath string) bool {
	if p := strings.TrimPrefix(filepath, prefix); len(p) < len(filepath) {
		name := path.Join(l.root, p)
		f, err := l.FileSystem.Open(name)
		if err != nil {
			return false
		}
		defer f.Close()
		stats, err := f.Stat()
		if err != nil {
			return false
		}
		if stats.IsDir() {
			if !l.indexes {
				index := path.Join(name, "index.html")
				f2, err := l.FileSystem.Open(index)
				if err != nil {
					return false
				}
				defer f2.Close()

				if _, err := f2.Stat(); err != nil {
					return false
				}
			}
		}
		return true
	}
	return false
}
