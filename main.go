package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	"time"

	rethink "github.com/dancannon/gorethink"
)

// GlobalConfig put together all configs
type GlobalConfig struct {
	Connect   *ConnectConfig `json:"connect"`
	*DBConfig `json:"db"`
	UserCnf   *UserConfig              `json:"user_cnf"`
	ChConfs   map[string]ChannelConfig `json:"channels"`
}

// DBConfig settings for RethinkDb
type DBConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// UserConfig for nickname and other things
type UserConfig struct {
	NickName string `json:"nick"`
}

// ChannelConfig settigns for separate channel
type ChannelConfig struct {
	ChanName string `json:"name"`
	URLs     bool   `json:"store_urls"`
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
	CreatedAt time.Time `gorethink:"created_at"`
	Author    string    `gorethink:"msg_author"`
	ChanName  string    `gorethink:"channel_name"`
	Formated  string    `gorethink:"formated_message"`
	HasURL    bool      `gorethink:"has_url"`
}

// ParseConfig read config file and return Config struct
func ParseConfig(path string) (conf GlobalConfig, err error) {
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
		if strings.Contains(line, "PING") {
			pong := strings.Split(line, "PING ")
			fmt.Fprintf(*c.Conn, "PONG %s\r\n", pong[1])
			continue
		}
		//TODO I have receive every 10-20 mins EOF, must think about it
		// Find - Twitch sendns me PINg every ~10-15 mins PING and i musy answer it
		if err != nil {
			log.Fatalf("while read %s \n", err)
		}
		out <- line
	}
}

// FormatMessage converts raw data to Message
// raw message looks like :feikga!feikga@feikga.tmi.twitch.tv PRIVMSG #test :No?
func formatMessage(raw string) (msg Message) {
	msg = Message{CreatedAt: time.Now(), HasURL: false}
	if strings.Contains(raw, "PRIVMSG") {
		message := strings.Split(raw, ".tmi.twitch.tv PRIVMSG #")
		msg.Author = strings.Split(strings.Split(message[0], "@")[0], "!")[1]
		t := strings.Split(message[1], " :")
		//TODO check why sometimes i haven't t[1]
		if len(t) >= 2 {
			msg.ChanName, msg.Formated = t[0], t[1]
			if strings.Contains(msg.Formated, "http") {
				msg.HasURL = true
			}
		} else {
			msg.ChanName = t[0]
		}
	} else {
		fmt.Fprintln(os.Stderr, raw)
	}
	return
}

// ConsumeData ouputed all data
func ConsumeData(data chan string, db *rethink.Session) {
	for i := range data {
		if !strings.Contains(i, "_bot") {
			msg := formatMessage(i)
			result, err := rethink.DB("Channels").Table("test").Insert(msg).RunWrite(session)
			if err != nil {
				log.Fatalf("error %s while inserting data %v \n", err, result.GeneratedKeys[0])
			}
		}
	}
}

var session *rethink.Session

func main() {
	conf, err := ParseConfig("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	session, err = rethink.Connect(rethink.ConnectOpts{
		Address:  conf.DBConfig.Host + ":" + conf.DBConfig.Port,
		Database: "Channels",
	})
	if err != nil {
		log.Fatalln(err)
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
	ConsumeData(data, session)
}
