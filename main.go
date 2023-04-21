package main

import (
	"encoding/json"

	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rotisserie/eris"
	"github.com/spf13/viper"
)

func main() {
	// config stuff
	viper.AddConfigPath(".")
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	err := viper.ReadInConfig()
	if err != nil {
		err = eris.Wrap(err, "failed to read config")
		log.Panic(err)
	}
	viper.AutomaticEnv()

	log.Infof("Starting %s...", viper.GetString("TRIAL_NAME"))

	// create rest client
	client := rest.New(rest.NewClient(viper.GetString("DISCORD_TOKEN")))

	// create snowflakes for ids
	channelId := snowflake.ID(viper.GetUint("CHANNEL_ID"))
	startId := snowflake.ID(viper.GetUint("START_MSG_ID"))
	endId := snowflake.ID(viper.GetUint("END_MSG_ID"))

	// new transcript manager
	transcript := NewTranscript(startId, endId, getNameOverrides())

	// get messages
	messages := client.GetMessagesPage(channelId, startId, 50)

	// add messages to transcript
	transcript.AddMessagesPage(messages)

	log.Info("Done getting messages")

	stats := transcript.GetStats()

	log.Info("Stats:")
	log.Infof("\tTotal messages: %d", stats.TotalMessages)
	log.Infof("\tTotal users: %d", stats.TotalUsers)
	log.Infof("\tStart date: %s", stats.StartDate)
	log.Infof("\tEnd date: %s", stats.EndDate)

	// // print transcript
	// transcript.PrintTranscript()

	// save transcript
	transcript.SaveTranscript()
}

// parse name override into map
func getNameOverrides() (nameOverride map[string]interface{}) {
	// parse override into map
	err := json.Unmarshal([]byte(viper.GetString("NAME_OVERRIDE")), &nameOverride)
	if err != nil {
		err = eris.Wrap(err, "failed to unmarshal name override")
		log.Panic(err)
	}
	return
}
