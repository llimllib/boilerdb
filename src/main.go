/**
 * Created with IntelliJ IDEA.
 * User: dvirsky
 * Date: 11/8/12
 * Time: 6:50 PM
 * To change this template use File | Settings | File Templates.
 */
package main

import (
	redis_adapter "adapters/redis"
	"db"
	"fmt"
	"log"
	"net"
	hash_table "plugins/hash_table"
	ptree "plugins/prefix_tree"
	simple "plugins/simple"
	system "plugins/system"
	"runtime"
)

///////////////////////////////////////////////////

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU()*2)
	database := db.InitGlobalDataBase()
	ht := new(hash_table.HashTablePlugin)
	smp := new(simple.SimplePlugin)
	ptree := new(ptree.PrefixTreePlugin)
	sys := new(system.SystemPlugin)
	database.RegisterPlugins(ht, smp, ptree, sys)

	_ = database.LoadDump()

	if true {
		adap := redis_adapter.RedisAdapter{}

		adap.Init(database)
		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:2000")
		err := adap.Listen(addr)

		if err != nil {
			fmt.Printf("Err: %s", err.Error())
			log.Fatal(err)
			return
		}

		fmt.Printf("Go..\n")
		adap.Start()

	}
	//fmt.Println(ret)
	for i := 0; i < 10; i++ {
		cmd := db.Command{"HSET", fmt.Sprintf("foo%d", i), [][]byte{[]byte("bar"), []byte("baz")}}
		_, _ = database.HandleCommand(&cmd)

	}
	//_, _ = database.Dump()

}
