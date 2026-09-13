module Native.Graphics.Raster.Native where

import Prelude
import Effect (Effect)
import Native.Graphics.Types (Color, Image, Transform)

foreign import data Surface :: Type
foreign import data NativePath :: Type
foreign import data TextLine :: Type
foreign import data FloatBuffer :: Type

foreign import critical :: forall a. Effect a -> Effect a
foreign import newSurface :: { width :: Int, height :: Int } -> Effect Surface
foreign import renderSurface :: Surface -> Effect Image
foreign import newImage :: { width :: Int, height :: Int } -> Effect Image
foreign import imageLength :: Image -> Int
foreign import readByte :: Image -> Int -> Effect Int
foreign import writeByte :: Image -> Int -> Int -> Effect Unit
foreign import newFloats :: Int -> Effect FloatBuffer
foreign import readFloat :: FloatBuffer -> Int -> Effect Number
foreign import writeFloat :: FloatBuffer -> Int -> Number -> Effect Unit
foreign import float32 :: Number -> Number
foreign import copyFloats :: FloatBuffer -> FloatBuffer -> Effect Unit
foreign import newPath :: Effect NativePath
foreign import moveTo :: { path :: NativePath, x :: Number, y :: Number } -> Effect Unit
foreign import lineTo :: { path :: NativePath, x :: Number, y :: Number } -> Effect Unit
foreign import quadTo :: { path :: NativePath, cx :: Number, cy :: Number, x :: Number, y :: Number } -> Effect Unit
foreign import cubicTo :: { path :: NativePath, cx1 :: Number, cy1 :: Number, cx2 :: Number, cy2 :: Number, x :: Number, y :: Number } -> Effect Unit
foreign import closePath :: NativePath -> Effect Unit
foreign import appendCircle :: { path :: NativePath, x :: Number, y :: Number, radius :: Number } -> Effect Unit
foreign import paint :: { surface :: Surface, path :: NativePath, transform :: Transform, fill :: Color, stroke :: Color, width :: Number, join :: Int, cap :: Int, evenOdd :: Boolean } -> Effect Unit
foreign import paintImage :: { surface :: Surface, image :: Image } -> Effect Unit
foreign import makeText :: { font :: String, size :: Number, color :: Color, text :: String } -> Effect { line :: TextLine, width :: Number, capHeight :: Number, ascent :: Number, descent :: Number }
foreign import paintText :: { surface :: Surface, transform :: Transform, line :: TextLine, x :: Number, y :: Number } -> Effect Unit
foreign import paintHatch :: { surface :: Surface, path :: NativePath, transform :: Transform, cell :: Transform, color :: Color, hatch :: { x0 :: Number, y0 :: Number, x1 :: Number, y1 :: Number } -> Effect NativePath } -> Effect Unit
