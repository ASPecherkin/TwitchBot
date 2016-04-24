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
	"runtime/pprof"
	"strings"
	"time"

	"flag"

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
	RawMsg    string    `gorethink:"raw_msg"`
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
func (c Channel) JoinChannel(out chan Message) {
	fmt.Fprintf(*c.Conn, "JOIN %s\r\n", "#"+c.Config.ChanName)
	reader := bufio.NewReader(*c.Conn)
	tp := textproto.NewReader(reader)
	for {
		line, err := tp.ReadLine()
		if strings.Contains(line, "PING") {
			pong := strings.Split(line, "PING ")
			fmt.Fprintf(*c.Conn, "PONG %s\r\n", pong[0])
			continue
		}
		//Twitch sendns me PING's every ~10-15 mins PING and i musy answer it
		if err != nil {
			log.Fatalf("while read %s \n", err)
		}
		// fmt.Println(c.Config.ChanName)
		msg := Message{RawMsg: line, CreatedAt: time.Now()}
		out <- msg
	}
}

func stringBetweenChars(text, f, l string) (between string) {
	first := strings.IndexAny(text, f)
	last := strings.IndexAny(text[first:], l) + first
	if first == -1 || last == -1 {
		return
	}
	between = text[first+1 : last]
	return
}

// FormatMessage converts raw data to Message
// raw message looks like :feikga!feikga@feikga.tmi.twitch.tv PRIVMSG #test :No?
func FormatMessage(input *Message) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Error : %v \n with message: %s \n ", p, input.RawMsg)
		}
	}()
	if strings.Contains(input.RawMsg, "PRIVMSG") {
		input.ChanName = stringBetweenChars(input.RawMsg, "#", " :")
		info := strings.Split(input.RawMsg, ".tmi.twitch.tv PRIVMSG #"+input.ChanName+" :")[0]
		message := strings.Split(input.RawMsg, ".tmi.twitch.tv PRIVMSG #"+input.ChanName+" :")[1]
		input.Author = stringBetweenChars(info, ":", "!")
		input.Formated = message
		if strings.Contains(input.Formated, "http") {
			input.HasURL = true
		}
	} else {
		fmt.Fprintln(os.Stderr, input.RawMsg)
	}
}

// ConsumeData ouputed all data
func ConsumeData(data chan Message, db *rethink.Session) {
	for i := range data {
		if !strings.Contains(i.RawMsg, "_bot") {
			FormatMessage(&i)
			result, err := rethink.DB("Channels").Table("test").Insert(&i).RunWrite(session)
			if err != nil {
				log.Fatalf("error %s while inserting data %v \n", err, result.GeneratedKeys)
			}
		}
	}
}

var session *rethink.Session
var cpuprofile = flag.String("cpup", "", "write cpu profile to file")
var memprofile = flag.String("memp", "", "write mem profile to file")

func main() {
	conf, err := ParseConfig("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		pprof.WriteHeapProfile(f)
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
	data := make(chan Message, 3*len(conf.ChConfs))
	defer close(data)
	for k := range conf.ChConfs {
		activeCh[k] = Channel{Conn: &conn, Config: conf.ChConfs[k]}
		go activeCh[k].JoinChannel(data)
	}
	ConsumeData(data, session)
}
