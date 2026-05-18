# Time

KiGo time module.

## Configuration

ENV variables:

APP_TIME_NAME - module name, default: `clock`
APP_TIME_PUBSUBURL - nats url, default: `nats://127.0.0.1:4222` 
APP_TIME_KIGONAME - KiGo pub sub topic, defautl: `KiGo`
APP_TIME_FORMAT - display format, default: `15:04:05`
APP_TIME_POSITION - position on the screen, default: `0`, TopCenter=0, TopLeft=1, TopRight=2

## Changes

Changes allowed during the lifecycle

- Format - change time format

