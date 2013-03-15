//
package replication

import (
	"db"
	"adapters/redis"
	"net"
	"logging"
	"strconv"
	"fmt"
	"strings"
	"bytes"
	"bufio"
	"encoding/gob"


)

const (
	STATE_OFFLINE = 0
	STATE_PENDING_SYNC = 1
	STATE_SYNC_IN_PROGRESS = 2
	STATE_LIVE = 3
)



//this represents the status of a master in a slave
type Master struct {
	State int
	Host string
	Port int
	Conn net.Conn
	decoder *gob.Decoder
	reader *bufio.ReadWriter


}

func (m *Master) String() string {

	return fmt.Sprintf("Master(%s:%d)", m.Host, m.Port)
}

// Connect to a master
func (m *Master)Connect() error {

	//make sure we don't connect without disconnecting
	if m.State != STATE_OFFLINE {
		logging.Warning("Could not connect to a connected master!")
		return fmt.Errorf("Trying to connect to an already connected master")
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", m.Host, m.Port))
	if err != nil {
		logging.Error("Could not connect to master %s: %s", m, err)
		return err
	}
	m.Conn = conn
	//set the state to "pending sync"
	m.State = STATE_PENDING_SYNC
	logging.Info("Successfuly connected to master %s", m)
	return nil
}



// Disconnect from a master
func (m *Master)Disconnect() error {


	m.State = STATE_OFFLINE
	err := m.Conn.Close()
	if err != nil {
		logging.Warning("Could not close conection to master: %s", err)
		return err
	}

	logging.Info("Disconnected master %s", m)

	return nil
}

func (m *Master)RunReplication()  {


	reader := bufio.NewReaderSize(m.Conn, 32768)
	writer := bufio.NewWriterSize(m.Conn, 32768)

	redis.SerializeResponse(&db.Command{"SYNC", "", make([][]byte, 0)}, writer)
	writer.Flush()
	mockSession :=  db.DB.NewSession(currentMaster.Conn.LocalAddr())
	mockSession.InChan = nil
	for m.State != STATE_OFFLINE {
		//read an parse the request
		cmd, _ := redis.ReadRequest(reader)

		if cmd.Command != "OK" {


			_, er := db.DB.HandleCommand(cmd, mockSession)
			if er != nil {
				logging.Warning("Error handling command: %s", er)
			}
		}


	}

	//stop the session
	mockSession.Stop()

	m.Disconnect()

}

var currentMaster *Master = nil


// Disconnect the slave from the current master
func disconnectMaster() {

	defer func() {
		e := recover()
		if e != nil {
			logging.Error("Could not disconnect master: %s", e)
			currentMaster = nil
		}

	}()
	if currentMaster != nil {

		err := currentMaster.Disconnect()
		if err != nil {
			logging.Warning("Could not close conection to master: %s", err)
		}
		currentMaster = nil
		logging.Info("Disconnected master")
	}


}

// Connect to a new master
func connectToMaster(host string, port int) error {


	//check to see if we already have a master
	if currentMaster != nil {
		if currentMaster.Host == host && currentMaster.Port == port {
			logging.Warning("Cannot connect to the same master")
			return fmt.Errorf("Trying to reconnect to the current master %s", currentMaster)
		}

		//disconnect from the current one first
		logging.Info("Disconnecting from current master first...")
		disconnectMaster()

	}


	m := &Master{
		Host: host,
		Port: port,
		State: STATE_OFFLINE,
		decoder: nil,
	}

	err := m.Connect()
	if err == nil {
		logging.Info("Setting new master to %s", m)
		currentMaster = m
	} else {
		return err
	}

	go currentMaster.RunReplication()


	return nil
}



func (m *Master) ReadValue(buf []byte) {


	if m.decoder == nil {

		m.decoder = gob.NewDecoder(m.reader)
	}

	m.reader.Writer.Write(buf)

	var se db.SerializedEntry
	err := m.decoder.Decode(&se)
	if err == nil {

		fmt.Println(se)
		db.DB.LoadSerializedEntry(&se)

	}


}



func HandleLOAD(cmd *db.Command, entry *db.Entry, session *db.Session) *db.Result {

	if currentMaster == nil {
		logging.Error("Got load while not connected to a master!")
		return db.NewResult(db.NewError(db.E_PLUGIN_ERROR))
	}

	l, e := strconv.Atoi(string(cmd.Args[1]))
	if e != nil {
		logging.Error("Could not read entry len: %s", e)
		return nil
	}

	var se db.SerializedEntry = db.SerializedEntry{
		Key: cmd.Key,
		Type: string(cmd.Args[0]),
		Len: uint64(l),
		Bytes: cmd.Args[2],
	}

	err := db.DB.LoadSerializedEntry(&se)
	if err != nil {

		logging.Error("Error loading entry: %s", e)
	}

	return nil
}

func HandleSLAVEOF(cmd *db.Command, entry *db.Entry, session *db.Session) *db.Result {
	address := cmd.Key
	logging.Info("Got slaveof to %s %s", cmd.Key, cmd.Args[0])

	if strings.ToUpper(cmd.Key) == "NO" && bytes.Equal(bytes.ToUpper(cmd.Args[0]), []byte("ONE")) {
		logging.Info("Disconnecting master")

		disconnectMaster()

		return db.ResultOK()
	}
	port, err := strconv.Atoi(string(cmd.Args[0]))
	//valiate port number
	if err !=nil || port < 1 || port > 65535 {

		return db.NewResult(db.NewPluginError("REPLICATION", "Invalid port number"))

	}


	remote :=    fmt.Sprintf("%s:%d", address, port)
	logging.Info("Connecting to master %s", remote)

	err = connectToMaster(address, port)

	if err != nil {
		logging.Warning("Aborting master connection: %s", err)
		return db.NewResult(db.NewPluginError("REPLICATION", fmt.Sprintf("Could not connect to master: %s", err)))
	}

	return db.ResultOK()
}

