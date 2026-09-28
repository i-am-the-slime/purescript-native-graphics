module Test.Native.Graphics.Metal.Text (main) where

import Prelude
import Data.Array as Array
import Data.Either (Either(..))
import Data.Maybe (Maybe(..))
import Effect (Effect)
import Effect.Exception (throw)
import Native.Graphics.Geometry as Matrix
import Native.Graphics.Metal.Text (decodeAtlas, textGeometry)
import Native.Graphics.Metal.Vertices as Vertices
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
  let middleVertices = Vertices.toArray middle.vertices
  unless (Array.length middleVertices == 96) $ throw "Unicode atlas glyph and synthetic bold must each emit one quad"
  expect "Center alignment includes synthetic bold advance" 0 89.0 middleVertices
  expect "Cap-middle baseline uses cap height, not ascent" 1 41.0 middleVertices
  expect "Bottom-origin atlas UVs flip vertically" 3 0.2 middleVertices
  expect "Inherited opacity multiplies text opacity" 7 0.4 middleVertices
  expect "Synthetic bold displaces the second run" 48 91.0 middleVertices
  let
    quad x0 x1 =
      [ x0
      , 41.0
      , 0.1
      , 0.2
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      , x1
      , 41.0
      , 0.6
      , 0.2
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      , x1
      , 59.0
      , 0.6
      , 0.8
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      , x0
      , 41.0
      , 0.1
      , 0.2
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      , x1
      , 59.0
      , 0.6
      , 0.8
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      , x0
      , 59.0
      , 0.1
      , 0.8
      , 1.0
      , 0.0
      , 0.0
      , 0.4
      ]
    expectedVertices = Vertices.toArray (Vertices.fromArray (quad 89.0 99.0 <> quad 91.0 101.0))
  unless (middleVertices == expectedVertices) $ throw "Packed glyph triangles must preserve every position, UV, color and run order"
  let
    skipped = textGeometry Matrix.identity 0.5 spec atlas font
      ( shaped
          { glyphs =
              [ { glyphID: 99, sourceIndex: 1, xOffset: 0.0, yOffset: 0.0, advance: 10.0 }
              , { glyphID: 7, sourceIndex: 0, xOffset: 1.0, yOffset: 2.0, advance: 22.0 }
              ]
          , width = 32.0
          }
      )
    expectedSkipped = Vertices.toArray (Vertices.fromArray (quad 94.0 104.0 <> quad 96.0 106.0))
  unless (Vertices.toArray skipped.vertices == expectedSkipped) $ throw "Skipped glyphs must advance the cursor in both normal and bold runs"
  unless (middle.screenPxRange == 4.0) $ throw "MSDF range must account for font size and backing scale"
  let upper = textGeometry Matrix.identity 1.0 (spec { baseline = Top }) atlas font shaped
  let lower = textGeometry Matrix.identity 1.0 (spec { baseline = Bottom }) atlas font shaped
  expect "Top baseline uses font ascent" 1 52.0 (Vertices.toArray upper.vertices)
  expect "Bottom baseline uses font descent" 1 30.0 (Vertices.toArray lower.vertices)
  case decodeAtlas (resources { width = 99 }) of
    Left _ -> pure unit
    Right _ -> throw "Atlas metadata must match uploaded image dimensions"
  where
  -- Compare the exact float32 values uploaded to Metal, without a tolerance.
  expect label index expected vertices =
    unless (Array.index vertices index == Array.head (Vertices.toArray (Vertices.fromArray [ expected ]))) (throw label)
