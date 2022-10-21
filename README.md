# aero
Go module for file sharing between devices. Master device starts the server and node devices connect to the server. Once connected, each device can communicate with each other to get shared file list and directly download the shared files.
## Usage

### Master device
```go
import (
	// ...
    "github.com/dhamith93/aero"
)

func main() {
    files := make([]aero.File, 0)
    files = append(files, aero.NewFile("/path/to/file"))
    aero := aero.New(
        aero.Device{
            Name:       "MASTER",
            Ip:         "192.168.1.2",
            Port:       "9000",
            SocketPort: "9001",
            Files:      files,
        },
        true,
    )

    aero.SetKey("key-for-jwt-tokens")

    var wg sync.WaitGroup
    wg.Add(2)
    go aero.StartGrpcServer()
    go aero.StartSocketServer()
    wg.Wait()
}
```

### Node device
```go
import (
	// ...
    "github.com/dhamith93/aero"
)

func main() {
    files := make([]aero.File, 0)
    files = append(files, aero.NewFile("/path/to/file"))
    aeroNew := aero.New(
        aero.Device{
            Name:       "Node",
            Ip:         "192.168.1.3",
            Port:       "9000",
            SocketPort: "9001",
            Files:      files,
        },
        true,
    )

    aeroNew.SetKey("key-for-jwt-tokens")

    // Register new device to server
    devices, err := aeroNew.SendInit(*aeroNew.Self, aero.Device{Port: "9000", Ip: "192.168.1.2"})
    if err != nil {
        fmt.Println(err.Error())
    }

    // Get list of devices with files
    devices, err = aeroNew.GetList()

    // Get status of a device
    status, err := aeroNew.GetStatus(devices[0])

    // Check if file available
    fileIdx := 0
    err := aeroNew.FetchFile(devices[0], fileIdx)

    // Download file and progress checking
    downloadId := aeroNew.Download(devices[0], fileIdx)

    for {
        if aeroNew.SocketServer.Downloads[downloadId].Progress == 100 && aeroNew.SocketServer.Downloads[downloadId].HashMatched {
            fmt.Println("file downloaded")
            break
        }
        if aeroNew.SocketServer.Downloads[downloadId].Error != nil {
            fmt.Println(aeroNew.SocketServer.Downloads[downloadId].Error.Error())
            break
        }
        fmt.Println(aeroNew.SocketServer.Downloads[downloadId].Progress)		
    }

    // Get messages/logs 
    fmt.Println(aeroNew.SocketServer.Messages.Get())
}
```
