package main

import (
	"testing"
	// "github.com/ASPecherkin/TwitchBot"
	"fmt"
)

func TestFormatMessage(t *testing.T) {
	fmt.Println("Run test for formating")
	var tests = []Message{
		{ChanName: "test", RawMsg: ":twitchnotify!twitchnotify@twitchnotify.tmi.twitch.tv PRIVMSG #test :NauruMaria just subscribed!"},
		{ChanName: "gnumme", RawMsg: ":grafreco!grafreco@grafreco.tmi.twitch.tv PRIVMSG #gnumme :@bladerunner_87, по идее прист, но что-то как-то не очень сейчас он играет"},
	}
	var want = []Message{
		{Author: "twitchnotify", Formated: "NauruMaria just subscribed!"},
		{Author: "grafreco", Formated: "@bladerunner_87, по идее прист, но что-то как-то не очень сейчас он играет"},
	}
	for k := range tests {
		FormatMessage(&tests[k])
		if tests[k].Author != want[k].Author && tests[k].Formated != want[k].Formated {
			t.Errorf("Got %v, want %v \n", tests[k], want[k])
		}
	}
}
