module Test.Native.Graphics.Raster (run) where

import Prelude
import Data.Array as Array
import Data.Either (either)
import Data.Foldable (for_)
import Data.Maybe (Maybe(..))
import Effect (Effect, forE)
import Effect.Exception (throw, throwException, try)
import Go.Concurrent as Concurrent
import Native.Graphics.Raster (rasterize)
import Native.Graphics.Raster.Native as N
import Native.Graphics.Raster.Pixels as Pixels
import Native.Graphics.Raster.Primitives (rectangle, transparent, white)
import Native.Graphics.Types (BlendMode(..), Color, Command(..), Drawing, FillRule(..), Image, LayerId(..), LineCap(..), LineJoin(..), PathOp(..))

red :: Color
red = { red: 1.0, green: 0.0, blue: 0.0, alpha: 1.0 }

drawing :: Array Command -> Drawing
drawing commands = { commands, layers: [ { global: false } ], composite: [ { source: LayerId 0, mask: Nothing, invertMask: false, blend: SourceOver } ], clear: transparent }

assertPixel :: String -> Image -> Int -> Int -> Int -> Array Int -> Effect Unit
assertPixel label image width x y expected =
  for_ (Array.mapWithIndex (\channel value -> { channel, value }) expected) \item -> do
    actual <- N.readByte image ((y * width + x) * 4 + item.channel)
    unless (actual == item.value) (throw (label <> ": channel " <> show item.channel <> " = " <> show actual <> ", expected " <> show item.value))

setPixel :: Image -> Int -> Array Int -> Effect Unit
setPixel image pixel values = for_ (Array.mapWithIndex (\channel value -> { channel, value }) values) \item -> N.writeByte image (pixel * 4 + item.channel) item.value

run :: Effect Unit
run = do
  cleared <- rasterize 100 100
    ( drawing
        [ Viewport { x: 0.0, y: 0.0, width: 50.0, height: 100.0 }
        , Clear white
        ]
    )
  for_ [ { x: 0, y: 0 }, { x: 99, y: 0 }, { x: 0, y: 99 }, { x: 99, y: 99 } ] \point ->
    assertPixel "clear ignores viewport" cleared 100 point.x point.y [ 255, 255, 255, 255 ]

  replacement <- rasterize 100 100
    ( drawing
        [ Viewport { x: 0.0, y: 0.0, width: 50.0, height: 50.0 }
        , Viewport { x: 0.0, y: 0.0, width: 100.0, height: 100.0 }
        , FillPath (rectangle { x: 75.0, y: 75.0, width: 10.0, height: 10.0 }) red
        ]
    )
  assertPixel "viewport replaces camera" replacement 100 80 80 [ 255, 0, 0, 255 ]

  clipped <- rasterize 100 100
    ( drawing
        [ Clear white
        , PushClip (rectangle { x: 25.0, y: 25.0, width: 50.0, height: 50.0 }) NonZero
        , FillPath (rectangle { x: 0.0, y: 0.0, width: 100.0, height: 100.0 }) red
        , PopClip
        ]
    )
  assertPixel "clip excludes outside" clipped 100 10 10 [ 255, 255, 255, 255 ]
  assertPixel "clip retains inside" clipped 100 50 50 [ 255, 0, 0, 255 ]

  hole <- rasterize 100 100
    ( drawing
        [ PushClip (rectangle { x: 0.0, y: 0.0, width: 100.0, height: 100.0 } <> rectangle { x: 25.0, y: 25.0, width: 50.0, height: 50.0 }) EvenOdd
        , Clear red
        , PopClip
        ]
    )
  assertPixel "even-odd outer region" hole 100 10 10 [ 255, 0, 0, 255 ]
  assertPixel "even-odd hole" hole 100 50 50 [ 0, 0, 0, 0 ]

  alpha <- rasterize 8 8 (drawing [ PushAlpha 0.5, Clear red, PopAlpha ])
  assertPixel "group alpha is premultiplied" alpha 8 4 4 [ 127, 0, 0, 127 ]

  layered <- rasterize 16 16
    { commands:
        [ Clear red
        , PushLayer (LayerId 1)
        , FillPath (rectangle { x: 0.0, y: 0.0, width: 8.0, height: 16.0 }) white
        , PopLayer
        ]
    , layers: [ { global: false }, { global: false } ]
    , composite: [ { source: LayerId 0, mask: Just (LayerId 1), invertMask: false, blend: SourceOver } ]
    , clear: transparent
    }
  assertPixel "layer recipe keeps masked half" layered 16 4 8 [ 255, 0, 0, 255 ]
  assertPixel "layer recipe excludes other half" layered 16 12 8 [ 0, 0, 0, 0 ]

  blurred <- rasterize 9 9
    ( drawing
        [ PushBlur 1.0
        , FillPath (rectangle { x: 4.0, y: 0.0, width: 1.0, height: 9.0 }) red
        , PopBlur
        ]
    )
  assertPixel "blur group captures real canvas pixels" blurred 9 3 4 [ 85, 0, 0, 85 ]
  assertPixel "blur group leaves distant pixels clear" blurred 9 0 4 [ 0, 0, 0, 0 ]

  src <- N.newImage { width: 1, height: 1 }
  dst <- N.newImage { width: 1, height: 1 }
  mask <- N.newImage { width: 1, height: 1 }
  setPixel src 0 [ 100, 50, 0, 128 ]
  setPixel dst 0 [ 40, 80, 120, 200 ]
  setPixel mask 0 [ 0, 0, 0, 128 ]
  Pixels.composite dst src (Just mask) false SourceOver
  assertPixel "masked premultiplied source-over" dst 1 0 0 [ 79, 84, 89, 213 ]
  setPixel dst 0 [ 40, 80, 120, 200 ]
  Pixels.composite dst src (Just mask) false Invert
  assertPixel "masked inversion" dst 1 0 0 [ 83, 103, 123, 213 ]
  setPixel dst 0 [ 40, 80, 120, 200 ]
  setPixel mask 0 [ 0, 0, 0, 255 ]
  Pixels.composite dst src (Just mask) true SourceOver
  assertPixel "inverted opaque mask excludes source" dst 1 0 0 [ 40, 80, 120, 200 ]

  impulse <- N.newImage { width: 5, height: 1 }
  setPixel impulse 2 [ 255, 0, 0, 255 ]
  Pixels.blur 5 1 impulse 1.0
  assertPixel "blur impulse outside window" impulse 5 0 0 [ 0, 0, 0, 0 ]
  for_ [ 1, 2, 3 ] \x -> assertPixel "three-box impulse" impulse 5 x 0 [ 85, 0, 0, 85 ]
  singleton <- N.newImage { width: 1, height: 1 }
  setPixel singleton 0 [ 63, 17, 4, 127 ]
  Pixels.blur 1 1 singleton 8.0
  assertPixel "blur extends singleton edges" singleton 1 0 0 [ 63, 17, 4, 127 ]

  let
    scene = drawing
      [ Clear white
      , StrokePath [ MoveTo 10.0 10.0, LineTo 50.0 50.0, LineTo 10.0 50.0, LineTo 50.0 10.0 ] red { width: 4.0, join: RoundJoin, cap: ButtCap }
      ]
  expected <- rasterize 64 64 scene
  completed <- Concurrent.newBufferedChan 8
  forE 0 8 \_ -> Concurrent.go do
    result <- try do
      actual <- rasterize 64 64 scene
      forE 0 (N.imageLength expected) \i -> do
        a <- N.readByte actual i
        b <- N.readByte expected i
        unless (a == b) (throw ("concurrent rasterization differs at byte " <> show i))
    Concurrent.writeChan completed result
  forE 0 8 \_ -> Concurrent.readChan completed >>= either throwException pure
