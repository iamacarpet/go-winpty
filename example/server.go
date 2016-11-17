package main

/*
 * Go-WinPTY example.
 * Provides a web based cmd or powershell prompt using xterm.js
 *
 * Usage:
 * GOOS=windows GOARCH=amd64 go build -o web-pty.exe server.go
 * Place winpty.dll and winpty-agent.exe in the same directory as web-pty.exe, then run.
 * .\web-pty.exe -cmd "powershell" -addr 127.0.0.1:9000 -static "static_folder_name"
 * .\web-pty.exe -cmd "cmd"
 *
 * Adapted from https://github.com/codenvy/che-websocket-terminal
 * Which was written by Al Tobey tobert@gmail.com
 * By Samuel Melrose sam@infitialis.com
 */

import (
    "io"
    "os"
    "log"
    "flag"
    "bufio"
    "bytes"
    "net/http"
    "unicode/utf8"
    "encoding/json"
    "github.com/gorilla/websocket"
    "github.com/iamacarpet/go-winpty"
)

var addrFlag, cmdFlag, staticFlag string

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1,
    WriteBufferSize: 1,
    CheckOrigin: func(r *http.Request) bool {
    return true
    },
}

type Message struct{
    Type string `json:"type"`
    Data json.RawMessage `json:"data"`
}

type wsPty struct {
    Pty		*winpty.WinPTY
    ws		*websocket.Conn
}

func (wp *wsPty) Start() {
    var err error
    // If you want to use a location other than the same folder for the DLL and exe
    // specify the path as the first param, e.g. winpty.Open(`C:\MYAPP\support`, cmdFlag)
    wp.Pty, err = winpty.Open("", cmdFlag)
    if err != nil {
    log.Fatalf("Failed to start command: %s\n", err)
    }
    //Set the size of the pty
    wp.Pty.SetSize(200, 60)
}

func (wp *wsPty) Stop() {
    wp.Pty.Close()

    wp.ws.Close()
}

func ptyHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
    log.Fatalf("Websocket upgrade failed: %s\n", err)
    }
    defer conn.Close()

    wp := wsPty{ ws: conn }

    wp.Start()

    go wp.writePump()
    wp.readPump()
}

func (wp *wsPty) readPump() {
    defer wp.Stop()

    for {
        mt, payload, err := wp.ws.ReadMessage()
        if err != nil {
            if err != io.EOF {
                log.Printf("conn.ReadMessage failed: %s\n", err)
                return
            }
        }
        var msg Message
        switch mt {
            case websocket.BinaryMessage:
                log.Printf("Ignoring binary message: %q\n", payload)
            case websocket.TextMessage:
                err := json.Unmarshal(payload, &msg)
                if err != nil {
                    log.Printf("Invalid message %s\n", err)
                    continue
                }
                switch msg.Type{
                    case "resize":
                        var size []float64
                        err := json.Unmarshal(msg.Data, &size)
                        if err != nil {
                            log.Printf("Invalid resize message: %s\n", err)
                        } else {
      	                    wp.Pty.SetSize(uint32(size[0]), uint32(size[1]))
                        }
                    case "data":
                        var dat string
                        err := json.Unmarshal(msg.Data, &dat)
                        if err != nil {
                            log.Printf("Invalid data message %s\n", err)
                        } else {
                            wp.Pty.StdIn.Write([]byte(dat));
                        }
                    default:
                        log.Printf("Invalid message type %d\n", mt)
                        return
                }
            default:
                log.Printf("Invalid message type %d\n", mt)
                return
        }
    }
}

func (wp *wsPty) writePump() {
    defer wp.Stop()

    buf := make([]byte, 8192)
    reader := bufio.NewReader(wp.Pty.StdOut)
    var buffer bytes.Buffer
    for {
        n, err := reader.Read(buf)
        if err != nil {
            log.Printf("Failed to read from pty master: %s", err)
            return
        }
        //read byte array as Unicode code points (rune in go)
        bufferBytes := buffer.Bytes()
        runeReader := bufio.NewReader(bytes.NewReader(append(bufferBytes[:],buf[:n]...)))
        buffer.Reset()
        i := 0
        for i < n {
            char, charLen, e := runeReader.ReadRune()
            if e != nil {
                log.Printf("Failed to read from pty master: %s", err)
                return
            }
            if char == utf8.RuneError {
                runeReader.UnreadRune()
                break
            }
            i += charLen
            buffer.WriteRune(char)
        }
        err = wp.ws.WriteMessage(websocket.TextMessage, buffer.Bytes())
        if err != nil {
            log.Printf("Failed to send UTF8 char: %s", err)
            return
        }
        buffer.Reset()
        if i < n {
            buffer.Write(buf[i:n])
        }
    }
}

func init() {
    cwd, _ := os.Getwd()
    flag.StringVar(&addrFlag, "addr", ":9000", "IP:PORT or :PORT address to listen on")
    flag.StringVar(&cmdFlag, "cmd", "cmd", "command to execute on slave side of the pty")
    flag.StringVar(&staticFlag, "static", cwd, "path to static content")
}

func main() {
    flag.Parse()

    http.HandleFunc("/pty", ptyHandler)

    // serve html & javascript
    http.Handle("/", http.FileServer(http.Dir(staticFlag)))

    err := http.ListenAndServe(addrFlag, nil)
    if err != nil {
        log.Fatalf("net.http could not listen on address '%s': %s\n", addrFlag, err)
    }
}
