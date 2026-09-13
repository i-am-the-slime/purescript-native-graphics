module Test.Native.Graphics.Font (run) where

import Prelude

import Control.Monad.Rec.Class (Step(..), tailRecM)
import Data.Either (Either(..))
import Data.Number as Number
import Effect (Effect)
import Effect.Exception (throw, try)
import Graphics.Canvas as Canvas
import Graphics.Canvas.Extra as Extra
import Native.Graphics (registerFont)
import Native.Graphics.Font as Font
import Native.Graphics.Types (Bytes)
import Web.Font.Loading (FontShorthand(..))
import Web.Font.Loading as Loading

assert :: String -> Boolean -> Effect Unit
assert label condition = unless condition (throw label)

-- Supply a real, loadable font fixture; no test-only native bindings are needed.
run :: Bytes -> Effect Unit
run data_ = do
  let
    first = "Native Font Regression First"
    second = "Native Font Regression Second"
    px = "italic 400 16px/1.5 \"" <> first <> "\""
    pt = "12pt '" <> first <> "'"
    content = "Measured advance"
    checkRegistered = Loading.checkFont (FontShorthand px)
  registerFont { name: first, data: data_, features: "" }
  registered <- checkRegistered
  assert "font check effects may be constructed before native registration" registered
  registerFont { name: second, data: data_, features: "" }
  missing <- tailRecM
    ( \index -> do
        let candidate = "Native Font Regression Missing " <> show index
        exists <- Loading.checkFont (FontShorthand ("16px '" <> candidate <> "'"))
        pure (if exists then Loop (index + 1) else Done candidate)
    )
    0

  selected <- Font.resolveFamily (" \"" <> missing <> "\" , '" <> second <> "' , \"" <> first <> "\" ")
  assert "family fallback selects the first registered quoted name" (selected == second)
  fallbackRegistered <- Loading.checkFont (FontShorthand ("16px '" <> missing <> "', '" <> first <> "'"))
  assert "font check uses ordered family fallback" fallbackRegistered
  invalidRegistered <- Loading.checkFont (FontShorthand first)
  assert "registered family alone is not a valid shorthand" (not invalidRegistered)

  canvas <- Extra.createOffscreenCanvas 80.0 30.0
  context <- Canvas.getContext2D canvas
  Canvas.setFont context px
  pixels <- Canvas.measureText context content
  assert "native shaping reports positive advance" (pixels.width > 0.0)
  Canvas.setFont context pt
  points <- Extra.measureTextInk context content
  assert "12pt and 16px produce identical advances" (points.width == pixels.width)
  assert "unavailable native ink bounds remain NaN" (Number.isNaN points.ascent && Number.isNaN points.descent)

  Canvas.setFont context "not a font shorthand"
  Canvas.setFont context ("font16px '" <> first <> "'")
  Canvas.setFont context (px <> "\n")
  retained <- Canvas.font context
  afterInvalid <- Canvas.measureText context content
  assert "invalid assignment preserves the prior font and measured advance"
    (retained == pt && afterInvalid.width == points.width)

  other <- Extra.createOffscreenCanvas 20.0 10.0
  otherContext <- Canvas.getContext2D other
  Canvas.setFont otherContext ("18.75px '" <> second <> "'")
  fractionalPixels <- Canvas.measureText otherContext content
  Canvas.setFont otherContext ("14.0625pt '" <> second <> "'")
  fractionalPoints <- Canvas.measureText otherContext content
  assert ("fractional px and pt sizes preserve the same measured advance: " <> show { pixels: fractionalPixels.width, points: fractionalPoints.width, reference: pixels.width })
    (fractionalPixels.width == fractionalPoints.width && fractionalPixels.width > pixels.width)
  Canvas.setCanvasWidth canvas 123.0
  Canvas.setCanvasHeight canvas 45.0
  width <- Canvas.getCanvasWidth canvas
  height <- Canvas.getCanvasHeight canvas
  otherWidth <- Canvas.getCanvasWidth other
  otherHeight <- Canvas.getCanvasHeight other
  sameContext <- Canvas.getContext2D canvas
  independentFont <- Canvas.font sameContext
  assert "canvas dimensions are mutable and isolated across canvases"
    (width == 123.0 && height == 45.0 && otherWidth == 20.0 && otherHeight == 10.0)
  assert "resizing and another context's font do not replace measurement font state" (independentFont == pt)

  let unavailable = "18px '" <> missing <> "'"
  Canvas.setFont context unavailable
  assigned <- Canvas.font context
  available <- Loading.checkFont (FontShorthand unavailable)
  assert "valid font assignment does not depend on resource availability" (assigned == unavailable && not available)
  failure <- try (Canvas.measureText context content)
  case failure of
    Left _ -> pure unit
    Right _ -> throw "measuring an unavailable family must not silently reuse the previous face"

  registerFont { name: missing, data: data_, features: "" }
  nowAvailable <- Loading.checkFont (FontShorthand unavailable)
  late <- Canvas.measureText context content
  Canvas.setFont otherContext ("18px '" <> first <> "'")
  reference <- Canvas.measureText otherContext content
  assert "later registration resolves the existing context without resetting its font"
    (nowAvailable && late.width == reference.width)
