module Test.Main (main) where

import Prelude

import Data.Array as Array
import Data.Maybe (Maybe(..))
import Effect (Effect)
import Effect.Console (log)
import Effect.Exception (catchException, message, throw)
import Native.Graphics (bytesFromBase64)
import Native.Window as Window
import Node.Encoding (Encoding(..))
import Node.FS.Sync as File
import Node.Process as Process
import Test.Native.Graphics.Font as Font
import Test.Native.Graphics.Metal.Geometry as Geometry
import Test.Native.Graphics.Metal.Text as Text
import Test.Native.Graphics.Raster as Raster

main :: Effect Unit
main = catchException (\error -> log (message error) *> Process.exit' 1) do
  arguments <- Process.argv
  path <- case Array.index arguments 2 of
    Just value -> pure value
    Nothing -> throw "usage: native-graphics-tests FONT.base64"
  encoded <- File.readTextFile UTF8 path
  Raster.run
  log "PASS raster pixels, groups, blur, masks and concurrency"
  when (Window.platform == "darwin") do
    Geometry.main
    log "PASS Metal clipping geometry"
  Text.main
  log "PASS Metal atlas and text placement"
  Font.run (bytesFromBase64 encoded)
  log "PASS native font selection and measurement"
