package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"strings"
	"time"
)

// Globalonfig put together all configs
type Globalonfig struct {
	Connect *ConnectConfig           `json:"connect"`
	UserCnf *UserConfig              `json:"user_cnf"`
	ChConfs map[string]ChannelConfig `json:"channels"`
}

// ChannelConfig settigns for separate channel
type ChannelConfig struct {
	ChanName string `json:"name"`
	URLs     bool   `json:"store_urls"`
}

// UserConfig for nickname and other things
type UserConfig struct {
	NickName string `json:"nick"`
}

// ConnectConfig settings for connecting to the twitch server
type ConnectConfig struct {
	URL      string `json:"host"`
	Port     string `json:"port"`
	UserName string `json:"user"`
	Password string `json:"oauth"`
}

// Channel settings for specific channel
type Channel struct {
	Conn   *net.Conn
	Config ChannelConfig
}

// Message every message in chat
type Message struct {
	CreatedAt  time.Time
	RawMessage string
	Author     string
	ChanName   string
	Formated   string
}

// ParseConfig read config file and return Config struct
func ParseConfig(path string) (conf Globalonfig, err error) {
	data, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatalf("Couldn't read config file: %s  \n", err)
		return
	}
	err = json.Unmarshal(data, &conf)
	return
}

// InitConnect init coonect to the twitch chat
func InitConnect(cfg ConnectConfig) (conn net.Conn, err error) {
	conn, err = net.Dial("tcp", cfg.URL+":"+cfg.Port)
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Fprintf(conn, "USER %s 8 * :%s\r\n", cfg.UserName, cfg.UserName)
	fmt.Fprintf(conn, "PASS %s\r\n", cfg.Password)
	fmt.Fprintf(conn, "NICK %s\r\n", cfg.UserName)
	return
}

// JoinChannel join specific channel with given nick
func (c Channel) JoinChannel(out chan string) {
	fmt.Fprintf(*c.Conn, "JOIN %s\r\n", "#"+c.Config.ChanName)
	reader := bufio.NewReader(*c.Conn)
	tp := textproto.NewReader(reader)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			log.Fatalf("while read %s \n", err)
			break
		}
		out <- line
	}
}

// FormatMessage converts raw data to Message
func formatMessage(raw string) (msg Message) {
	msg = Message{CreatedAt: time.Now(), RawMessage: raw}
	if strings.Contains(raw, "PRIVMSG") {
		message := strings.Split(raw, ".tmi.twitch.tv PRIVMSG #")
		msg.Author = strings.Split(strings.Split(message[0], "@")[0], "!")[0]
		msg.Formated = strings.Split(message[1], " :")[1]
		msg.ChanName = strings.Split(message[1], " :")[0]
	}
	return
}

// ConsumeData ouputed all data
func ConsumeData(data chan string) {
	for i := range data {
		msg := formatMessage(i)
		fmt.Printf("%s@%s: %s \n", msg.ChanName, msg.Author, msg.Formated)
	}
}

func main() {
	conf, err := ParseConfig("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := InitConnect(*conf.Connect)
	if err != nil {
		log.Fatal(err)
	}
	activeCh := make(map[string]Channel, len(conf.ChConfs))
	data := make(chan string, 3*len(conf.ChConfs))
	for k, v := range conf.ChConfs {
		activeCh[k] = Channel{Conn: &conn, Config: v}
		go activeCh[k].JoinChannel(data)
	}
	ConsumeData(data)
}
