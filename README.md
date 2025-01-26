# Bluesky Link Stream

Connects to the Bsky jetstream and filters for posts with links.

Runs at -> https://bluesky-link-stream.jacob.earth/

Hat tip to https://github.com/veekaybee/gitfeed for inspiration.

## Docker commands

```docker
docker build -t atfun .

docker run -p 8080:8080 atfun
```