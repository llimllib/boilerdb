/**
 * Created with IntelliJ IDEA.
 * User: dvirsky
 * Date: 12/3/12
 * Time: 1:04 AM
 * To change this template use File | Settings | File Templates.
 */
package json
import (
	gob "encoding/gob"
	"encoding/json"
	"db"
	"strings"
	"logging"
	"bytes"
	"fmt"

)

const T_JSON string = "JSON"


func (jq *JsonQuery)Serialize(g *gob.Encoder) error {

	err := g.Encode(jq)

	return err
}



type JSONPlugin struct {

}

func parseJSONPath(path string)[]string {
	arg := strings.Replace(path, "[", ".", -1)
	arg = strings.Replace(arg, "]", "", -1)
	arg = strings.TrimRight(arg, ".")
	return strings.Split(arg, ".")
}
func HandleJSET(cmd *db.Command, entry *db.Entry, session *db.Session) *db.Result {

	jo, ok := entry.Value.(*JsonQuery)
	if !ok {
		return db.NewResult(db.NewPluginError("JSON", fmt.Sprintf("This does not appear to be a JSON struct")))
	}
	var data interface{}
	dec := json.NewDecoder(strings.NewReader(string(cmd.Args[1])))
	err := dec.Decode(&data)
	if err != nil {
		logging.Info("Error decoding data: %s (%s)", err,string(cmd.Args[1]) )
		return db.NewResult(db.NewPluginError("JSON", fmt.Sprintf("Invalid JSON: %s", err)))
	}


	//set the root object
	if string(cmd.Args[0]) == "." {
		jo.Blob = data
		err = nil
	} else {
		err = jo.Set(data, parseJSONPath(string(cmd.Args[0]))...)
	}

	if err == nil {
		return db.NewResult("OK")
	}
	logging.Warning("Unable to set: %s", err)
	return db.NewResult(db.NewPluginError("JSON", fmt.Sprintf("Could not set: %s", err)))

}

func HandleJGET(cmd *db.Command, entry *db.Entry, session *db.Session) *db.Result {

	if entry == nil {
		return nil
	}


	jq := entry.Value.(*JsonQuery)
	ret, err := json.Marshal(jq.Blob)

	if err != nil {
		return db.NewResult(db.NewPluginError("JSON", fmt.Sprint(err)))

	}

	return db.NewResult(string(ret))



}

func HandleJQUERY(cmd *db.Command, entry *db.Entry, session *db.Session) *db.Result {

	if entry == nil {
		return nil
	}
	jq := entry.Value.(*JsonQuery)

	path := parseJSONPath(string(cmd.Args[0]))

	ret, err := jq.String(path...)
	if err != nil {
		return db.NewResult(db.NewPluginError("JSON", fmt.Sprint(err)))
	}
	return db.NewResult(ret)
}



func (p *JSONPlugin)CreateObject(commandName string) (*db.Entry, string) {

	return &db.Entry{ Value: NewQuery(make(map[string]interface{})) }, T_JSON

}

//deserialize and create a db entry
func (p *JSONPlugin)LoadObject(buf []byte, t string) *db.Entry {

	fmt.Println(t)
	if t == T_JSON {
		var s JsonQuery
		buffer := bytes.NewBuffer(buf)
		dec := gob.NewDecoder(buffer)
		err := dec.Decode(&s)
		if err != nil {
			logging.Warning("Could not deserialize oject: %s", err)
			return nil
		}

		return &db.Entry{ Value: &s }

	}else {
		logging.Error("Could not load object - invalid type %s", t)
	}

	return nil
}

func (p *JSONPlugin)GetManifest() db.PluginManifest {

	return db.PluginManifest {

		Name: "JSON",

		Commands:  []db.CommandDescriptor {
			db.CommandDescriptor{
				CommandName: "JSET",
				MinArgs: 2,	MaxArgs: 2,
				Handler: HandleJSET,
				CommandType: db.CMD_WRITER,
			},
			db.CommandDescriptor{
				CommandName: "JGET",
				MinArgs: 0,	MaxArgs: 0,
				Handler: HandleJGET,
				CommandType: db.CMD_READER,
			},
			db.CommandDescriptor{
				CommandName: "JQUERY",
				MinArgs: 1,	MaxArgs: 1,
				Handler: HandleJQUERY,
				CommandType: db.CMD_READER,
			},

		},
		Types: []string{ T_JSON, },

	}
}

func (p* JSONPlugin) String() string {
	return "JSON"
}
