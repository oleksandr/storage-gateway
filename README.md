# storage-gateway

Simple RESTful gateway for MongoDB's GridFS. The main reason to implement this gateway was the outdated Nginx GridFS module (https://github.com/mdirolf/nginx-gridfs). Unfortunately it seemed to be too hard to reanimate that project in comparison to writing a _microservice_ for accessing GridFS :)

## Installation

    go install github.com/oleksandr/storage-gateway

## Configuration

See `env.sh.sample` for all required configuration for the `storage-gateway` executable.

## API

Available operations on buckets and objects:

 * POST /objects
 * HEAD /objects/<id>
 * GET /objects/<id>/meta
 * GET /objects/<id>
 * HEAD /bucket/<name>
 * GET /bucket/<name>

Updating/deleting the object/bucket is currently work in progress.

## Authentication

No authentication is built inside of this gateway. The recommended way is to use hide it behind a reverse proxy. For example, you can use `nginx` with `--with-http_auth_request_module` and use the following configuration snippet:

        location ~ ^/downloads/(?<section>.*) {
            # If you want to disable uploads
            if ( $request_method !~ ^(GET|HEAD)$ ) {
                return 405;
            }
            auth_request            /_auth/;
            error_page 401 /401/;

            # Proceed to storage gateway
            proxy_pass              http://storage_gateway/$section;

            more_clear_headers 'WWW-Authenticate';
            more_clear_headers 'Server';
        }

        # Handle sub-requests to verify current session.
        # This location should be used only from other
        location = /_auth/ {
            proxy_pass              http://identity_provider/session/current;
            proxy_redirect          off;
            proxy_pass_request_body off;
            proxy_set_header        Content-Length "";
            proxy_set_header        X-Original-URI $request_uri;
            proxy_set_header        X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header        Host $http_host;
        }

        location = /401/ {
            return 401 '{"message": "Unauthorized","status": 401}';
            more_set_headers "Content-Type: application/json; charset=UTF-8";
            more_clear_headers 'WWW-Authenticate';
            more_clear_headers 'Server';
        }


## Examples of consuming this API with HTTPie

Some examples of consuming the API using HTTPie CLI.

### Upload a file (and assing it to a bucket)

    $ http --form post :6000/objects object@/Users/alex/Projects/4tree/sit-dev-layout/sit-storage-gateway/my_photo.png content_type="image/png" filename="cockpit-test.json" extra.bucket="tests"
     
    HTTP/1.1 201 Created
    Content-Length: 290
    Content-Type: application/json; charset=utf-8
    Date: Thu, 25 Jun 2015 11:24:41 GMT
    {
        "chunk_size": 261120,
        "content_type": "image/png",
        "created_on": "2015-06-25T13:24:41.178+02:00",
        "extra": {
            "bucket": "tests",
            "cid": "c570279c-1b2c-11e5-9cb9-28cfe91e6af1"
        },
        "filename": "my_photo.png",
        "id": "558be4f91bdaffc7a4000001",
        "md5": "5b3874fcb2c1863c8111110aba19c1d3",
        "size": 16101
    }

### Get bucket content

    $ http get localhost:6000/buckets/tests 
    HTTP/1.1 200 OK
    Connection: keep-alive
    Content-Encoding: gzip
    Content-Type: application/json; charset=utf-8
    Date: Thu, 25 Jun 2015 12:11:57 GMT
    Transfer-Encoding: chunked
    Vary: Accept-Encoding
    {
        "name": "tests",
        "objects": [
            {
                "chunk_size": 261120,
                "content_type": "plain/text",
                "created_on": "2015-06-24T18:30:56.041+02:00",
                "extra": {
                    "cid": "some-correlation-id"
                },
                "filename": "README.md",
                "id": "558adb3f1bdaff30a3000001",
                "md5": "655b0ad0d3882fc2bfe06a512159dde4",
                "size": 4304
            },
            {
                "chunk_size": 261120,
                "content_type": "image/png",
                "created_on": "2015-06-25T10:18:43.817+02:00",
                "extra": {
                    "cid": "some-correlation-id-123"
                },
                "filename": "funny.png",
                "id": "558bb9631bdaff7440000001",
                "md5": "739233bb2bc219878c1b8a6fbe8b19b5",
                "size": 804504
            },
            ...
        ]
    }

### Check if bucket exists

    $ http head localhost:6000/buckets/tests
    HTTP/1.1 302 Found
    Connection: keep-alive
    Content-Type: application/json; charset=utf-8
    Date: Thu, 25 Jun 2015 12:11:28 GMT

### Download an object

    $ http -d get localhost:6000/objects/558be4f91bdaffc7a4000001
    HTTP/1.1 200 OK
    Connection: keep-alive
    Content-Disposition: inline; filename="cockpit-test.json"
    Content-Type: application/json
    Date: Thu, 25 Jun 2015 12:13:08 GMT
    Server: nginx
    Transfer-Encoding: chunked
    Vary: Accept-Encoding
    Downloading to "cockpit-test.json"
    Done. 15.72 kB in 0.00044s (34.57 MB/s)
