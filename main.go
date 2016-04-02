package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
)

// Config settings for main twitch server
type Config struct {
	URL      string `json:"host"`
	Port     string `json:"port"`
	UserName string `json:"user"`
	Password string `json:"oauth"`
}

// Channel settings for specific channel
type Channel struct {
	Conn net.Conn
}

// ParseConfig read config file and return Config struct
func ParseConfig(path string) (conf Config, err error) {
	data, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatalf("Couldn't read config file: %s  \n", err)
		return
	}
	err = json.Unmarshal(data, &conf)
	fmt.Println(conf)
	return
}

// InitConnect init coonect to the twitch chat
func InitConnect(cfg Config) (conn net.Conn, err error) {
	conn, err = net.Dial("tcp", cfg.URL+":"+cfg.Port)
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Fprintf(conn, "USER %s 8 * :%s\r\n", cfg.UserName, cfg.UserName)
	fmt.Fprintf(conn, "PASS %s\r\n", cfg.Password)
	return
}

// JoinChannel join specific channel with given nick
func (c *Channel) JoinChannel(nick string, name string, out chan string) {
	fmt.Fprintf(c.Conn, "NICK %s\r\n", nick)
	fmt.Fprintf(c.Conn, "JOIN %s\r\n", "#gnumme")
	reader := bufio.NewReader(c.Conn)
	tp := textproto.NewReader(reader)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			log.Fatalln(err)
			break
		}
		out <- line
	}
}

// ConsumeData ouputed all data
func ConsumeData(data chan string) {
	for i := range data {
		fmt.Println(i)
	}
}

func main() {
	conf, err := ParseConfig("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := InitConnect(conf)
	if err != nil {
		log.Fatal(err)
	}
	data := make(chan string, 3)
	go ConsumeData(data)
	channel := Channel{Conn: conn}
	channel.JoinChannel("galadrian", "galadrian", data)
}
