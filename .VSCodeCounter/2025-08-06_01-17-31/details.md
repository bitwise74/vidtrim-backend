# Details

Date : 2025-08-06 01:17:31

Directory /home/eryk/Code/Go/src/video-api

Total : 42 files,  2517 codes, 132 comments, 525 blanks, all 3174 lines

[Summary](results.md) / Details / [Diff Summary](diff.md) / [Diff Details](diff-details.md)

## Files
| filename | language | code | comment | blank | total |
| :--- | :--- | ---: | ---: | ---: | ---: |
| [video-api/api/ffmpeg\_process.go](/video-api/api/ffmpeg_process.go) | Go | 91 | 0 | 21 | 112 |
| [video-api/api/ffmpeg\_progress.go](/video-api/api/ffmpeg_progress.go) | Go | 39 | 0 | 12 | 51 |
| [video-api/api/ffmpeg\_start.go](/video-api/api/ffmpeg_start.go) | Go | 33 | 1 | 10 | 44 |
| [video-api/api/file\_delete.go](/video-api/api/file_delete.go) | Go | 103 | 0 | 18 | 121 |
| [video-api/api/file\_fetch\_bulk.go](/video-api/api/file_fetch_bulk.go) | Go | 104 | 1 | 17 | 122 |
| [video-api/api/file\_serve.go](/video-api/api/file_serve.go) | Go | 82 | 1 | 16 | 99 |
| [video-api/api/file\_upload.go](/video-api/api/file_upload.go) | Go | 255 | 6 | 52 | 313 |
| [video-api/api/root\_heartbeat.go](/video-api/api/root_heartbeat.go) | Go | 8 | 0 | 4 | 12 |
| [video-api/api/root\_validate.go](/video-api/api/root_validate.go) | Go | 8 | 0 | 4 | 12 |
| [video-api/api/router.go](/video-api/api/router.go) | Go | 127 | 15 | 37 | 179 |
| [video-api/api/user\_delete.go](/video-api/api/user_delete.go) | Go | 1 | 0 | 1 | 2 |
| [video-api/api/user\_fetch.go](/video-api/api/user_fetch.go) | Go | 46 | 2 | 11 | 59 |
| [video-api/api/user\_guest.go](/video-api/api/user_guest.go) | Go | 8 | 23 | 8 | 39 |
| [video-api/api/user\_login.go](/video-api/api/user_login.go) | Go | 88 | 0 | 19 | 107 |
| [video-api/api/user\_register.go](/video-api/api/user_register.go) | Go | 99 | 0 | 23 | 122 |
| [video-api/api/user\_stats.go](/video-api/api/user_stats.go) | Go | 26 | 0 | 8 | 34 |
| [video-api/cloudflare/r2\_client.go](/video-api/cloudflare/r2_client.go) | Go | 49 | 1 | 11 | 61 |
| [video-api/config-example.toml](/video-api/config-example.toml) | TOML | 28 | 29 | 16 | 73 |
| [video-api/config/config.go](/video-api/config/config.go) | Go | 137 | 12 | 40 | 189 |
| [video-api/db/conn.go](/video-api/db/conn.go) | Go | 18 | 0 | 6 | 24 |
| [video-api/go.mod](/video-api/go.mod) | Go Module File | 86 | 0 | 5 | 91 |
| [video-api/go.sum](/video-api/go.sum) | Go Checksum File | 358 | 0 | 1 | 359 |
| [video-api/main.go](/video-api/main.go) | Go | 23 | 0 | 8 | 31 |
| [video-api/middleware/body.go](/video-api/middleware/body.go) | Go | 26 | 1 | 6 | 33 |
| [video-api/middleware/jwt.go](/video-api/middleware/jwt.go) | Go | 102 | 2 | 18 | 122 |
| [video-api/middleware/request\_id.go](/video-api/middleware/request_id.go) | Go | 11 | 3 | 4 | 18 |
| [video-api/middleware/turnstile.go](/video-api/middleware/turnstile.go) | Go | 51 | 0 | 11 | 62 |
| [video-api/model/file.go](/video-api/model/file.go) | Go | 16 | 15 | 4 | 35 |
| [video-api/model/guest.go](/video-api/model/guest.go) | Go | 11 | 0 | 2 | 13 |
| [video-api/model/stats.go](/video-api/model/stats.go) | Go | 9 | 0 | 2 | 11 |
| [video-api/model/user.go](/video-api/model/user.go) | Go | 8 | 0 | 3 | 11 |
| [video-api/security/argon.go](/video-api/security/argon.go) | Go | 68 | 2 | 19 | 89 |
| [video-api/service/duration.go](/video-api/service/duration.go) | Go | 21 | 0 | 7 | 28 |
| [video-api/service/ffmpeg.go](/video-api/service/ffmpeg.go) | Go | 161 | 3 | 35 | 199 |
| [video-api/service/progress.go](/video-api/service/progress.go) | Go | 3 | 0 | 3 | 6 |
| [video-api/service/thumbnail.go](/video-api/service/thumbnail.go) | Go | 23 | 5 | 9 | 37 |
| [video-api/util/float\_to\_timestamp.go](/video-api/util/float_to_timestamp.go) | Go | 16 | 0 | 6 | 22 |
| [video-api/util/rand\_str.go](/video-api/util/rand_str.go) | Go | 28 | 4 | 6 | 38 |
| [video-api/validators/email.go](/video-api/validators/email.go) | Go | 18 | 2 | 6 | 26 |
| [video-api/validators/file.go](/video-api/validators/file.go) | Go | 78 | 3 | 20 | 101 |
| [video-api/validators/password.go](/video-api/validators/password.go) | Go | 20 | 0 | 7 | 27 |
| [video-api/validators/processing\_opts.go](/video-api/validators/processing_opts.go) | Go | 30 | 1 | 9 | 40 |

[Summary](results.md) / Details / [Diff Summary](diff.md) / [Diff Details](diff-details.md)