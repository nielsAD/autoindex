autoindex
=========
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

Lightweight `go` web server that provides a searchable directory index. Optimized for handling large numbers of files (100k+) and remote file systems (with high latency) through a continously updated directory cache.

[Live demo](https://archive.toom.io/)

#### Features:

* Lightweight single-page application (`~8KB html/css/js`)
* Responsive design
* Recursive file search
* Directory cache (`sqlite`)
* Sitemap support


Usage
-----

`./autoindex [options]`

|     Flag   |  Type  | Description |
|------------|--------|-------------|
|`-a`        |`string`|TCP network address to listen for connections|
|`-d`        |`string`|Database location|
|`-r`        |`string`|Root directory to serve|
|`-i`        |`string`|Refresh interval|
|`-forwarded`|`bool`  |Trust X-Real-IP and X-Forwarded-For headers|
|`-cached`   |`bool`  |Serve everything from cache (rather than search/recursive queries only)|

#### Example

`./autoindex -a=":4000" -i=5m -d=/tmp/autoindex.db -cached -r=/mnt/storage`
