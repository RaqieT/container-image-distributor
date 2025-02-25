# Container Image Distributor
The main purposes of this project is to learn [golang](https://go.dev/) syntax, but prepare something useful for my daily tasks.
I came with idea to prepare simple CLI tool that will automate:

1. Pulling image.
2. Generation of destination container image tag from source image tag.
3. Pushing image.

Whenever I needed to "repush" image from 
`registryA.com/team-a/application-x:123` to 
`registryB.com/team-b/external/team-a/application-x:25-02-2025` I was forced to prepare and execute following set of commands:
```
docker pull registryA.com/team-a/application-x:123
docker tag registryA.com/team-a/application-x:123 registryB.com/team-b/external/team-a/application-x:25-02-2025
docker push registryB.com/team-b/external/team-a/application-x:25-02-2025
```

Using this app I can do it with single command and without doing some manual string concatenation:
```shell
./cid -i registryA.com/team-a/application-x:123 -d teamB -t 25-02-2025
```
and following `config.json` used by app:
```json
{
  "repositories": [
    {
      "name": "source",
      "registry": "registryA.com",
      "suffix": "team-a"
    },
    {
      "name": "target",
      "additionalNames": ["teamB"],
      "registry": "registryB.com",
      "suffix": "team-b",
      "destinationMappings": {
        "application-x": "external/team-a/application-x"
      }
    }
  ]
}
```

Example output:
```
Generated destination image: registryB.com/team-b/external/team-a/application-x:25-02-2025
Do you agree to tag & push it? [type y to confirm]: y
2025/02/25 18:04:25 Pulling image...
2025/02/25 18:06:42 123: Pulling from team-a/application-x
123456789012: Pulling fs layer
...

2025/02/25 18:06:42 Tagging image...
2025/02/25 18:06:42 
2025/02/25 18:06:42 Pushing image...
2025/02/25 18:08:50 The push refers to repository [registryB.com/team-b/external/team-a/application-x]
123456789012: Preparing
...
25-02-2025: digest: sha256:deadbeef... size: 123

```

## Requirements
Docker/Podman installed.

## Build
```shell
go build -o cid
```

## Run
```
./cid
```

## Help
```
Usage of ./cid:
  -c string
        alias for -container-tool (default "docker")
  -container-tool string
        podman/docker (default "docker")
  -d string
        alias for -destination
  -destination-repository string
        destination repository which will be picked from "config.json" based on repository "name" or "additionalNames". If starts with "!" repositories from "config.json" will be ignored, it will execute push to specified destination followed by "!".
  -f    alias for -force
  -force
        push image without asking for destination path verification
  -i string
        alias for -image
  -image string
        image that will be used
  -override-tag string
        override image tag
  -t string
        alias for -override-tag
```
