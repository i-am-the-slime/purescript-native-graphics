module Test.Native.Graphics.Metal.Text (main) where

import Prelude
import Data.Array as Array
import Data.Either (Either(..))
import Data.Maybe (Maybe(..))
import Effect (Effect)
import Effect.Exception (throw)
import Native.Graphics.Geometry as Matrix
import Native.Graphics.Metal.Text (decodeAtlas, textGeometry)
import Native.Graphics.Types (TextAlign(..), TextBaseline(..))

main :: Effect Unit
main = do
  let
    resources =
      { atlasJSON: "{\"atlas\":{\"distanceRange\":4,\"size\":40,\"width\":100,\"height\":100,\"yOrigin\":\"bottom\"},\"variants\":[{\"glyphs\":[{\"index\":7,\"unicode\":128512,\"planeBounds\":{\"left\":0,\"bottom\":-0.2,\"right\":0.5,\"top\":0.7},\"atlasBounds\":{\"left\":10,\"bottom\":20,\"right\":60,\"top\":80}}]}]}"
      , width: 100
      , height: 100
      , fonts: [ { atlasVariant: 0, unicodeKeys: true } ]
      }
  atlas <- case decodeAtlas resources of
    Left message -> throw message
    Right value -> pure value
  font <- case Array.head atlas.fonts of
    Nothing -> throw "Expected decoded font"
    Just value -> pure value
  let
    spec =
      { x: 100.0
      , y: 50.0
      , text: "\x1F600"
      , font: ""
      , fontID: 0
      , size: 20.0
      , align: AlignCenter
      , baseline: Middle
      , boldOffset: 0.1
      , color: { red: 1.0, green: 0.0, blue: 0.0, alpha: 0.8 }
      }
  let
    shaped =
      { glyphs: [ { glyphID: 7, sourceIndex: 0, xOffset: 1.0, yOffset: 2.0, advance: 22.0 } ]
      , width: 22.0
      , capHeight: 14.0
      , ascent: 18.0
      , descent: 4.0
      , backingScale: 2.0
      }
  let middle = textGeometry Matrix.identity 0.5 spec atlas font shaped
  unless (Array.length middle.vertices == 96) $ throw "Unicode atlas glyph and synthetic bold must each emit one quad"
  expect "Center alignment includes synthetic bold advance" 0 89.0 middle.vertices
  expect "Cap-middle baseline uses cap height, not ascent" 1 41.0 middle.vertices
  expect "Bottom-origin atlas UVs flip vertically" 3 0.2 middle.vertices
  expect "Inherited opacity multiplies text opacity" 7 0.4 middle.vertices
  expect "Synthetic bold displaces the second run" 48 91.0 middle.vertices
  unless (middle.screenPxRange == 4.0) $ throw "MSDF range must account for font size and backing scale"
  let upper = textGeometry Matrix.identity 1.0 (spec { baseline = Top }) atlas font shaped
  let lower = textGeometry Matrix.identity 1.0 (spec { baseline = Bottom }) atlas font shaped
  expect "Top baseline uses font ascent" 1 52.0 upper.vertices
  expect "Bottom baseline uses font descent" 1 30.0 lower.vertices
  case decodeAtlas (resources { width = 99 }) of
    Left _ -> pure unit
    Right _ -> throw "Atlas metadata must match uploaded image dimensions"
  where
  expect label index expected vertices = unless (Array.index vertices index == Just expected) (throw label)
