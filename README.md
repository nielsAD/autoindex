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


Behind nginx
------------

Example configuration for running `autoindex` behind an `nginx` proxy.

```
upstream autoindex {
    server 127.0.0.1:4000;
    keepalive 8;
}

map $request_uri $request_basename {
    ~/(?<captured_request_basename>[^/?]*)(?:\?|$) $captured_request_basename;
}

map $request_uri $idx_path {
    ~/(?<captured_request_args>\?.*)?$                                $captured_request_args;
    ~/(?<captured_request_path>[^?]*)(?<captured_request_args>\?.*)?$ $captured_request_path/$captured_request_args;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name _;

    root /opt/autoindex/public;

    location / {
        rewrite ^/(.*)/$ /$1 permanent;
        try_files $uri /index.html;
        expires 1y;
    }

    location = /index.html {
        http2_push /idx/$idx_path;
        expires 1d;
    }

    location ^~ /dl/ {
        limit_rate 1m;
        add_header Content-Disposition 'attachment; filename="$request_basename"';
        add_header X-Robots-Tag "noindex, nofollow, nosnippet, noarchive";
    }

    location ~ ^(/idx/|/urllist.txt) {
        proxy_pass https://autoindex;
    }
}
```
