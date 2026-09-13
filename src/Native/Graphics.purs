module Native.Graphics
  ( module Types
  , rasterize
  , capture
  , encodePng
  , encodePngBytes
  , bytesFromBase64
  , registerFont
  ) where

import Prelude

import Effect (Effect)
import Native.Graphics.Ebiten as Ebiten
import Native.Graphics.Font as Font
import Native.Graphics.Raster as Raster
import Native.Graphics.Types (Bytes, Drawing, Image)
import Native.Graphics.Types (BlendMode(..), Bytes, CirclePatternSpec, Color, Command(..), Composite, Drawing, FillRule(..), Image, LatticeSpec, Layer, LayerId(..), LineCap(..), LineJoin(..), Path, PathOp(..), Point, Rect, StrokeStyle, TextAlign(..), TextBaseline(..), TextSpec, Transform) as Types

rasterize :: Int -> Int -> Drawing -> Effect Image
rasterize = Raster.rasterize

capture :: Int -> Int -> Drawing -> Effect Image
capture width height drawing = do
  render <- Ebiten.create
  captureImpl width height (\target -> render target drawing false)

registerFont :: { name :: String, data :: Bytes, features :: String } -> Effect Unit
registerFont = Font.registerFont

foreign import captureImpl :: Int -> Int -> (Image -> Effect Unit) -> Effect Image

foreign import encodePng :: Image -> Effect (Array Int)

foreign import encodePngBytes :: Image -> Effect Bytes

foreign import bytesFromBase64 :: String -> Bytes
