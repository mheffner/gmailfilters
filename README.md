# gmailfilters

A tool to sync Gmail filters from a config file to your account.

> **NOTE:** This makes it so the single configuration file is the only way to
   add filters to your account, meaning if you add a filter via the UI and do not
   also add it in your config file, the next time you run this tool on your
   outdated config, the filter you added _only_ in the UI will be deleted.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Installation](#installation)
    - [Binaries](#binaries)
    - [Via Go](#via-go)
- [Usage](#usage)
- [Example Filter File](#example-filter-file)
- [Setup](#setup)
  - [Gmail](#gmail)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->


## Installation

#### Binaries

For installation instructions from binaries please visit the [Releases Page](https://github.com/jessfraz/gmailfilters/releases).

#### Via Go

```console
$ go get github.com/jessfraz/gmailfilters
```

## Usage

```console
$ gmailfilters -h
gmailfilters -  A tool to sync Gmail filters from a config file to your account.

Usage: gmailfilters <command>

Flags:

  --creds-file    Gmail credential file (or env var GMAIL_CREDENTIAL_FILE) (default: <none>)
  -d, --debug     enable debug logging (default: false)
  -e, --export    export existing filters (default: false)
  --filters-file  Filters file (or env GMAIL_FILTERS_FILE) (default: <none>)
  --labels-file   Labels file (or env GMAIL_LABELS_FILE) (default: <none>)
  --token-file    Gmail oauth token file (default: /tmp/token.json)

Commands:

  version  Show the version information.
```

To start with, export your existing Gmail filters using the `--export` option.

## Example Filter File

```toml
[[Filter]]
[Filter.Criteria]
Query = "list:\"*.librato.github.com>\""
[Filter.Action]
Label = "INBOX/github/librato"
Archive = true

[[Filter]]
[Filter.Criteria]
Query = "list:(<cloud-computing.googlegroups.com>)"
[Filter.Action]
Label = "INBOX/cloud-computing"
Archive = true

[[Filter]]
[Filter.Criteria]
Query = "list:(ltrace-devel.lists.alioth.debian.org)"
[Filter.Action]
Label = "INBOX/ltrace-devel"
Archive = true

[[Filter]]
[Filter.Criteria]
Query = "list:statsite.armon.github.com"
[Filter.Action]
Label = "Devel"

[[Filter]]
[Filter.Criteria]
Query = "listid:containers.lists.linux-foundation.org"
[Filter.Action]
Label = "INBOX/linux/conts"
Archive = true


[[Filter]]
[Filter.Criteria]
From = "@world-comp.org"
Subject = "Congress"
[Filter.Action]
Label = ""
Delete = true
```

### Example labels file

```toml
[[Label]]
Id = "Label_9"
Name = "INBOX/apple/drivers"
Type = "user"

[[Label]]
Id = "Label_10"
Name = "INBOX/apple/kernel"
Type = "user"

[[Label]]
Id = "Label_11"
Name = "INBOX/apple/scitech"
Type = "user"

[[Label]]
Id = "Label_14"
Name = "INBOX/archimedes/bugs"
Type = "user"

[[Label]]
Id = "Label_1234"
Name = "STATSDAPP"
BackgroundColor = "#f691b2"
TextColor = "#994a64"
LabelListVisibility = "show"
MessageListVisibility = "show"
Type = "user"
```

## Setup

### Gmail

1. Enable the API: To get started using Gmail API, you need to 
    first create a project in the 
    [Google API Console](https://console.developers.google.com),
    enable the API, and create credentials.

    Follow the instructions 
    [for step enabling the API here](https://developers.google.com/gmail/api/quickstart/go).

2. With the credentials file saved to `creds.json`, run the following:
```
gmailfilters --creds-file creds.json --token-file token.json
```
3. Follow the URL prompt and permit the token access. Copy the provided token JSON string.
4. Paste the token JSON back in the terminal, it should now be saved as `token.json`.