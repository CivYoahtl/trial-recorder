package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rotisserie/eris"
	"github.com/spf13/viper"
)

type MsgAttachment struct {
	ID          snowflake.ID
	Filename    string
	ContentType string
	URL         string
	Size        int
}

type Msg struct {
	Content     string
	ID          snowflake.ID
	Attachments []MsgAttachment
}

type MsgBlock struct {
	UserId   snowflake.ID
	Name     string
	Messages []Msg
}

type Transcript struct {
	Blocks       []MsgBlock
	StartMsgID   snowflake.ID
	EndMsgID     snowflake.ID
	nameOverride map[string]interface{}
}

type TranscriptStats struct {
	TotalMessages int
	TotalUsers    int
	StartDate     string
	EndDate       string
}

// Adds a message to the transcript
func (t *Transcript) AddMessage(m discord.Message) {
	// we get messages in the reverse order they were send,
	// so we need to add them in the reverse order we get them

	msg := Msg{
		Content:     m.Content,
		ID:          m.ID,
		Attachments: []MsgAttachment{},
	}

	if len(m.Attachments) > 0 {
		for _, a := range m.Attachments {
			msg.Attachments = append(msg.Attachments, MsgAttachment{
				ID:          a.ID,
				Filename:    a.Filename,
				ContentType: *a.ContentType,
				URL:         a.URL,
				Size:        a.Size,
			})
		}
	}

	// if first block is the same user
	if len(t.Blocks) > 0 && t.Blocks[0].UserId == m.Author.ID {
		// just put the message in the first block
		t.Blocks[0].Messages = append([]Msg{msg}, t.Blocks[0].Messages...)

		return
	}

	var name string

	// check if we have a name override
	if t.nameOverride[m.Author.ID.String()] != nil {
		name = t.nameOverride[m.Author.ID.String()].(string)
	} else {
		name = m.Author.Username
		t.nameOverride[m.Author.ID.String()] = name
	}

	// create a new block
	msgBlock := MsgBlock{
		UserId:   m.Author.ID,
		Name:     name,
		Messages: []Msg{msg},
	}
	t.Blocks = append([]MsgBlock{msgBlock}, t.Blocks...)
}

// Gets messages from a page and adds them to the transcript
func (t *Transcript) AddMessagesPage(messages rest.Page[discord.Message]) {
	count := 0

	// page through messages
	for messages.Next() {
		log.Infof("Processing page %d...", count+1)

		for _, m := range messages.Items {
			t.AddMessage(m)
		}

		count++
	}

	t.Sort()

	// trim excess messages
	t.RemoveExcessMessages()
}

// brute force sorting after inital sort
func (t *Transcript) Sort() {
	sort.Slice(t.Blocks, func(i, j int) bool {
		iValue := t.Blocks[i].Messages[0].ID.Time().Compare(t.Blocks[j].Messages[0].ID.Time())

		if iValue == 0 {
			// same time??
			return false
		} else if iValue == 1 {
			// i comes before j
			return false
		} else {
			// j comes before i
			return true
		}
	})
}

// removes all messages after end message
func (t *Transcript) RemoveExcessMessages() {
	atEnd := false

	// remove messages after end id
	for i := 0; i < len(t.Blocks); i++ {

		for j := 0; j < len(t.Blocks[i].Messages); j++ {
			msg := t.Blocks[i].Messages[j]

			// if found end message
			if msg.ID == t.EndMsgID {
				atEnd = true

				// if not at end of array, remove all messages after end message
				if j+1 < len(t.Blocks[i].Messages) {
					t.Blocks[i].Messages = t.Blocks[i].Messages[:j+1]
				}
			}
		}

		// if at end, remove all blocks after end block
		if atEnd {
			t.Blocks = t.Blocks[:i+1]
		}
	}
}

// Gets basic stats about the transcript
func (t *Transcript) GetStats() TranscriptStats {
	totalMessages := 0
	seenUsers := []snowflake.ID{}
	startTime := t.Blocks[0].Messages[0].ID.Time().Format("Mon, 02 Jan 2006 15:04:05 MST")
	endTime := t.Blocks[len(t.Blocks)-1].Messages[len(t.Blocks[len(t.Blocks)-1].Messages)-1].ID.Time().Format("Mon, 02 Jan 2006 15:04:05 MST")

	for _, b := range t.Blocks {

		// check if user has been seen
		seenUser := false
		for _, u := range seenUsers {
			if b.UserId == u {
				seenUser = true
				continue
			}
		}
		if !seenUser {
			seenUsers = append(seenUsers, b.UserId)
		}

		// tally up msg stats
		totalMessages += len(b.Messages)
	}

	return TranscriptStats{
		TotalMessages: totalMessages,
		TotalUsers:    len(seenUsers),
		StartDate:     startTime,
		EndDate:       endTime,
	}
}

// prints transcript to console
func (t *Transcript) PrintTranscript() {
	println("----------------- Transcript -----------------")
	for _, b := range t.Blocks {
		println(b.Name)
		for _, m := range b.Messages {
			println(m.Content)
		}
		println("---")
	}
}

// writes transcript to a file
func (t *Transcript) SaveTranscript() {
	log.Info("Saving transcript...")

	trialName := viper.GetString("TRIAL_NAME")
	// format trial name for use as filename
	trailNameFormated := t.FormatFileName(trialName)

	folderPath := fmt.Sprintf("transcripts/%s", trailNameFormated)

	// check if folder exists
	if _, err := os.Stat(folderPath); eris.Is(err, os.ErrNotExist) {
		// if eror is that folder doesn't exist, create it
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			// failed to create folder
			err := eris.Wrap(err, "failed to create folder")
			log.Panic(err)
		}
	}

	// create file
	f, err := os.Create(folderPath + "/" + trailNameFormated + ".md")
	if err != nil {
		eris.Wrap(err, "failed to create file")
		log.Panic(err)
	}

	// remember to close the file
	defer f.Close()

	// scaffold the file
	writeToFile(f, "# "+trialName)
	writeToFile(f, "## Case")
	writeToFile(f, "_REPLACE ME: need a sumarry of the case here_")
	writeToFile(f, "## Proceedings")

	// write transcript
	for _, b := range t.Blocks {
		writeToFile(f, "**"+b.Name+"**:")
		writeToFile(f, "")
		// println(b.Name)
		for _, m := range b.Messages {

			attachments := ""

			for _, a := range m.Attachments {
				// construct attachment path for use on the website
				attachmentPath := fmt.Sprintf("../../assets/judiciary/%s/%s", trailNameFormated, a.Filename)

				attachments += fmt.Sprintf("![%s](%s)\n", a.Filename, attachmentPath)
				t.saveAttachment(a, folderPath)
			}

			// preprocess message to handle newlines
			content := strings.ReplaceAll(m.Content, "\n", "\n> ")
			content = t.replaceMentions(content)

			writeToFile(f, "> "+content)
			// writeToFile(f, "")

			// only write attachments if there are any
			if len(attachments) > 0 {
				writeToFile(f, "")
				writeToFile(f, attachments)
			}
		}
		writeToFile(f, "")
	}

	log.Info("Transcript saved!")
}

func (t *Transcript) saveAttachment(attachment MsgAttachment, folderPath string) {
	// Create blank file
	file, err := os.Create(folderPath + "/" + attachment.Filename)
	if err != nil {
		err := eris.Wrap(err, "failed to create attachment file")
		log.Fatal(err)
	}
	defer file.Close()

	// http client to download attachment
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	// Put content on file
	resp, err := client.Get(attachment.URL)
	if err != nil {
		err := eris.Wrap(err, "failed to download attachment")
		log.Fatal(err)
	}
	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		err := eris.Wrap(err, "failed to write attachment to file")
		log.Fatal(err)
	}

	log.Debugf("Saved attachment: %s (%d)", attachment.Filename, size)
}

// creates a new transcript
func NewTranscript(startMsgID, endMsgID snowflake.ID, nameOverride map[string]interface{}) *Transcript {
	return &Transcript{
		StartMsgID:   startMsgID,
		EndMsgID:     endMsgID,
		Blocks:       []MsgBlock{},
		nameOverride: nameOverride,
	}
}

// simpe util to append a line to a file
func writeToFile(file *os.File, content string) {
	_, err := file.WriteString(content + "\n")
	if err != nil {
		eris.Wrap(err, "failed to write to file")
		log.Panic(err)
	}
}

func (t *Transcript) replaceMentions(content string) string {
	outerPart := regexp.MustCompile("<@!*&*|>")

	return regexp.MustCompile("<@!*&*[0-9]+>").ReplaceAllStringFunc(content, func(s string) string {
		idStr := outerPart.ReplaceAllString(s, "")

		idNum, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			eris.Wrap(err, "failed to parse id")
			log.Panic(err)
		}

		id := snowflake.ID(idNum)

		if val, ok := t.nameOverride[id.String()].(string); ok {
			return "`@" + val + "`"
		} else {
			return s
		}
	})
}

// formats the trial name for use as the filename of the transcript
func (t *Transcript) FormatFileName(trialName string) (fileName string) {
	// force lowercase
	fileName = strings.ToLower(trialName)

	// clear whitespace
	fileName = strings.ReplaceAll(fileName, " ", "_")

	// remove special characters
	fileName = strings.ReplaceAll(fileName, ".", "")
	fileName = strings.ReplaceAll(fileName, ",", "")
	fileName = strings.ReplaceAll(fileName, ":", "")
	fileName = strings.ReplaceAll(fileName, ";", "")

	return
}
