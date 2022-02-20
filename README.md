# A toolkit for working with protobuf and gRPC in Go

[![ci](https://github.com/foxygoat/protog/actions/workflows/cicd.yaml/badge.svg?branch=master)](https://github.com/foxygoat/protog/actions/workflows/cicd.yaml?branch=master)
[![Godoc](https://img.shields.io/badge/godoc-ref-blue)](https://pkg.go.dev/foxygo.at/protog)
[![Slack chat](https://img.shields.io/badge/slack-gophers-795679?logo=slack)](https://gophers.slack.com/messages/foxygoat)

protog is a toolkit for Google's protobuf and gRPC projects. It contains a
couple of Go packages and command line tools.

It is developed against the protobuf-go v2 API (google.golang.org/protobuf)
which simplifies a lot of reflection-based protobuf code compared to v1
(github.com/golang/protobuf).

Build, test and install with `make`. See further options with `make help`.

## pb

`pb` is a CLI tool for converting proto messages between different formats.

Sample usage:

    # create base-massage.pb binary encoded proto message file from input JSON
    pb -P cmd/pb/testdata/pbtest.pb -o base-message.pb BaseMessage '{"f" : "some_field"}'

    # convert binary encoded proto message from pb file back to JSON
    pb -P cmd/pb/testdata/pbtest.pb -O json BaseMessage @base-message.pb

Output:

    {
      "f": "some_field"
    }
