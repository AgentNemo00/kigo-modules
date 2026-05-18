# Time

KiGo text module.

## Configuration

ENV variables:

APP_TEXT_NAME - module name, default: `text`
APP_TEXT_PUBSUBURL - nats url, default: `nats://127.0.0.1:4222` 
APP_TEXT_KIGONAME - KiGo pub sub topic, defautl: `KiGo`
APP_TEXT_POSITION - position on the screen, default: `0`, TopLeft=0, TopCenter=01, TopRight=2, MidLeft=3, MidCenter=4, MidRight=5, BottomLeft=6, BottomCenter=7, BottomRight=8
APP_TEXT_DIRECTION - Text direction, default: `0`, Right=0, Left=1, Down=2, Up=3

## Changes

Changes allowed during the lifecycle

- Text - change text
- Position - change position of the text
- Direction - change direction of the text

