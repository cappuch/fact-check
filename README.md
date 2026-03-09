# fact checking discord bot

exa + llm = fact checker for politics and stuff.

## how to set up

install go.
run go mod tidy.
run go build fact-check .

supply api keys. pretty important obv. sampel format in .env.example.

```
DISCORD_TOKEN=
EXA_API_KEY=
OPENAI_API_KEY=
OPENAI_BASE_URL=
OPENAI_MODEL_ID=
```

then just... run it

## comments
there's a db, intended for a future front-end version too, so people aren't locked to discord.