package main

import (
	"WnS/parseur"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"os"
	"sync"
)

type Node struct {
	conn *websocket.Conn
	prev *Node
	next *Node
}

type File struct {
	content []byte
	mutex   sync.Mutex
}

type Context struct {
	watcher         *fsnotify.Watcher
	wg              *sync.WaitGroup
	config          *Configuration
	interruptHandle chan os.Signal
	done            chan bool
	cache           map[string]File
}

type Configuration struct {
	directory string
}

type LinkedList struct {
	head *Node
}

var reload = make(chan bool, 1)

const fragment = `<script>
const eventSource = new EventSource('http://localhost:8080/events');
eventSource.onmessage = (event) => {
	eventSource.close();
	location.reload(true);
};
</script>`

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers to allow all origins. You may want to restrict this to specific origins in a production environment.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	<-reload
	w.Write([]byte("data: null\n\n"))
	w.(http.Flusher).Flush()
	<-r.Context().Done()
}

func handleWatcherEvents(c *Context) {
	for {
		select {
		case event, ok := <-c.watcher.Events:
			if !ok {
				return
			}

			writeEvent := event.Op&fsnotify.Write == fsnotify.Write
			createEvent := event.Op&fsnotify.Create == fsnotify.Create

			if writeEvent || createEvent {
				mappedFilename := event.Name[len(c.config.directory)+1:]
				createContent(c, mappedFilename)
				reload <- true
			}
		case _, ok := <-c.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func createContext(directory string) (*Context, error) {
	var context = &Context{}

	watcher, err := fsnotify.NewWatcher()
	context.cache = make(map[string]File)
	context.interruptHandle = make(chan os.Signal, 1)
	context.done = make(chan bool, 1)
	context.watcher = watcher
	context.wg = new(sync.WaitGroup)
	context.config = &Configuration{directory: directory}
	return context, err
}

func enumerateDir(c *Context) []string {
	suffixes := []string{".html", ".js"}
	files := make([]string, 0)
	k, _ := os.ReadDir(c.config.directory)
	for _, z := range k {
		if !z.IsDir() && endswith(z.Name(), suffixes) {
			files = append(files, z.Name())
		}
	}

	return files
}

func cache(c *Context, files []string) {
	suffix := []string{".html"}
	for _, file := range files {
		if endswith(file, suffix) {
			println("caching", file)
			createContent(c, file)

		}
	}
}

func createContent(c *Context, file string) {
	dat, err := os.ReadFile(c.config.directory + "/" + file)

	if err != nil {
		return
	}

	data := string(dat)

	p := parseur.NewParser(data)
	tags := p.GetTags()

	for _, t := range tags {
		if t.Name == "head" {
			c.cache[file] = File{[]byte(data[:t.Body.Start] + fragment + data[t.Body.Start:]), sync.Mutex{}}
			break
		}
	}
}

func endswith(name string, suffixes []string) bool {
	length := len(name)

	for _, suffix := range suffixes {
		offset := length - len(suffix)
		if offset <= 0 {
			continue
		}

		if name[offset:] == suffix {
			return true
		}
	}

	return false
}

func handleInterrupt(context *Context) {
	<-context.interruptHandle
	println("interrupted")
	context.done <- true
}

func main() {
	println("init")
	p := "static"

	if len(os.Args) > 1 {
		p = os.Args[1]
	}

	context, err := createContext(p)

	if err != nil {
		return
	}

	files := enumerateDir(context)

	cache(context, files)

	if err != nil {
		log.Fatal(err)
		return
	}

	defer context.watcher.Close()
	err = context.watcher.Add(context.config.directory)

	go handleInterrupt(context)
	go handleWatcherEvents(context)

	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write(context.cache["index.html"].content)
		} else {
			http.ServeFile(w, r, context.config.directory+r.URL.Path)
		}
	})

	http.HandleFunc("/events", eventsHandler)

	addr := ":8080"

	err = http.ListenAndServe(addr, nil)

	if err != nil {
		log.Fatal(err)
	}
}
