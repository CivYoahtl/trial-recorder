# Yoahtl Trial Recorder

A simple util to easily record trials with the Yoahtl court system as markdown files for use in other systems, like the [website](https://civyoahtl.github.io/).

## Usage

1. Create a Discord bot and invite it to your server.
2. Set the environment variables listed below.
3. Run the bot.
   ```bash
   go run .
   ```
   or just run the docker image

The bot will then record the messages in the channel specified by `CHANNEL_ID` from the message with ID `START_MSG_ID` to the message with ID `END_MSG_ID` and save them to a file named `<trial name>.md` in the transcripts folder, where `<trial name>` is the value of the `TRIAL_NAME` environment variable.

## Environment Variables

- `DISCORD_TOKEN` - The token to use to connect to Discord.
- `CHANNEL_ID` - The ID of the channel to record from.
- `START_MSG_ID` - The ID of the message to start recording from. This is exclusive, so the message with this ID will not be recorded.
- `END_MSG_ID` - The ID of the message to end recording at. This is inclusive, so the message with this ID will be recorded.
- `TRIAL_NAME` - The name of the trial to record.

(The bot will also take an .env file in the current directory if it exists.)

## Go Version

1.20
