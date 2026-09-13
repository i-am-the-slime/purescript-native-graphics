module Native.Window
  ( Input
  , Output
  , Options
  , FontConfig
  , Color
  , Appearance
  , Config
  , configure
  , deviceScaleFactor
  , platform
  , run
  ) where

import Prelude
import Data.Either (either)
import Data.Int as Int
import Effect (Effect)
import Effect.Exception (throwException, try)
import Native.Graphics.Ebiten as Ebiten
import Native.Graphics.Ebiten.Primitives as GPU
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Metal as Metal
import Native.Graphics.Raster as Raster
import Native.Graphics.Types (Bytes, Drawing, Image, Rect, Transform)

type Input =
  { deltaSeconds :: Number
  , mouseX :: Number
  , mouseY :: Number
  , mouseDown :: Boolean
  , mouseInside :: Boolean
  , spaceDown :: Boolean
  , leftDown :: Boolean
  , rightDown :: Boolean
  , homeDown :: Boolean
  , endDown :: Boolean
  , tDown :: Boolean
  , qDown :: Boolean
  , escapeDown :: Boolean
  , closeRequested :: Boolean
  }

type Output =
  { drawing :: Drawing
  , overlay :: Drawing
  , quit :: Boolean
  , frameWidth :: Int
  , frameHeight :: Int
  , windowWidth :: Int
  , windowHeight :: Int
  , dragExclusion :: { x :: Number, y :: Number, width :: Number, height :: Number }
  }

type Options =
  { backend :: String
  , title :: String
  , width :: Int
  , height :: Int
  , frameWidth :: Int
  , frameHeight :: Int
  , fps :: Int
  }

type FontConfig = { data :: Bytes, atlasVariant :: Int, unicodeKeys :: Boolean }

type Color = { red :: Number, green :: Number, blue :: Number, alpha :: Number }

type Appearance =
  { name :: String
  , background :: Color
  , opaque :: Boolean
  , titlebarTransparent :: Boolean
  , titleHidden :: Boolean
  , fullSizeContentView :: Boolean
  , movableByBackground :: Boolean
  , backdrop :: Boolean
  , glass :: Boolean
  , cornerRadius :: Number
  , glassStyle :: Int
  , tint :: Color
  , material :: Int
  , blendingMode :: Int
  , state :: Int
  }

type Config =
  { fonts :: Array FontConfig
  , atlasPNG :: Bytes
  , atlasJSON :: Bytes
  , iconPNG :: Bytes
  , quitMenuTitle :: String
  , quitKeyEquivalent :: String
  , sampleCount :: Int
  , appearance :: Appearance
  }

type NativeInput =
  { deltaSeconds :: Number
  , mouseX :: Number
  , mouseY :: Number
  , mouseDown :: Boolean
  , spaceDown :: Boolean
  , leftDown :: Boolean
  , rightDown :: Boolean
  , homeDown :: Boolean
  , endDown :: Boolean
  , tDown :: Boolean
  , qDown :: Boolean
  , escapeDown :: Boolean
  , closeRequested :: Boolean
  , frameWidth :: Int
  , frameHeight :: Int
  , viewportWidth :: Number
  , viewportHeight :: Number
  }

type MetalRenderer =
  { render :: Drawing -> Drawing -> Number -> Number -> Int -> Int -> Effect Unit
  , viewportRect :: Number -> Number -> Int -> Int -> Rect -> Rect
  }

run :: Options -> (Input -> Effect Output) -> Effect Unit
run options callback =
  let
    handleInput = callback <<< frameInput
  in
    if options.backend == "metal" then do
      render <- Metal.create
      let settings = options { fps = if options.fps > 0 then options.fps else 60 }
      runMetalImpl settings handleInput { render, viewportRect }
    else if options.backend == "cpu" then
      runEbitenImpl options handleInput \target drawing overlay -> do
        GPU.clearImage target
        rasterFrame target drawing
        rasterFrame target overlay
    else do
      render <- Ebiten.create
      runEbitenImpl options handleInput \target drawing overlay -> do
        render target drawing false
        render target overlay true

rasterFrame :: Image -> Drawing -> Effect Unit
rasterFrame target drawing = do
  size <- GPU.dimensions target
  pixels <- Raster.rasterize size.width size.height drawing
  texture <- GPU.uploadImage pixels
  outcome <- try (GPU.drawImage { target, source: texture, alpha: 1.0 })
  GPU.disposeImage texture
  either throwException pure outcome

frameTransform :: Number -> Number -> Int -> Int -> Transform
frameTransform width height frameWidth frameHeight =
  Geometry.viewport width height
    { x: 0.0, y: 0.0, width: Int.toNumber frameWidth, height: Int.toNumber frameHeight }

viewportRect :: Number -> Number -> Int -> Int -> Rect -> Rect
viewportRect width height frameWidth frameHeight rect =
  let
    transform = frameTransform width height frameWidth frameHeight
    origin = Geometry.apply transform rect.x rect.y
  in
    { x: origin.x, y: origin.y, width: rect.width * transform.a, height: rect.height * transform.d }

frameInput :: NativeInput -> Input
frameInput input =
  let
    transform = frameTransform input.viewportWidth input.viewportHeight input.frameWidth input.frameHeight
    x = if transform.a > 0.0 then (input.mouseX - transform.tx) / transform.a else input.mouseX
    y = if transform.d > 0.0 then (input.mouseY - transform.ty) / transform.d else input.mouseY
  in
    { deltaSeconds: input.deltaSeconds
    , mouseX: x
    , mouseY: y
    , mouseDown: input.mouseDown
    , mouseInside: x >= 0.0 && y >= 0.0 && x < Int.toNumber input.frameWidth && y < Int.toNumber input.frameHeight
    , spaceDown: input.spaceDown
    , leftDown: input.leftDown
    , rightDown: input.rightDown
    , homeDown: input.homeDown
    , endDown: input.endDown
    , tDown: input.tDown
    , qDown: input.qDown
    , escapeDown: input.escapeDown
    , closeRequested: input.closeRequested
    }

foreign import configure :: Config -> Effect Unit
foreign import deviceScaleFactor :: Effect Number
foreign import platform :: String
foreign import runMetalImpl :: Options -> (NativeInput -> Effect Output) -> MetalRenderer -> Effect Unit
foreign import runEbitenImpl :: Options -> (NativeInput -> Effect Output) -> (Image -> Drawing -> Drawing -> Effect Unit) -> Effect Unit
