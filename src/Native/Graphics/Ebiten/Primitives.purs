module Native.Graphics.Ebiten.Primitives where

import Prelude
import Effect (Effect)
import Native.Graphics.Types (Color, Image, Point)

foreign import data NativePath :: Type
foreign import data Shader :: Type
foreign import data Face :: Type
foreign import data FaceSource :: Type

-- Premultiplied device bytes; conversion from drawing colors is PureScript.
type RGBA8 = { red :: Int, green :: Int, blue :: Int, alpha :: Int }
type FaceMetrics = { ascent :: Number, descent :: Number, capHeight :: Number }

type Uniform = { name :: String, values :: Array Number }
type ShaderDraw =
  { target :: Image
  , shader :: Shader
  , images :: Array Image
  , uniforms :: Array Uniform
  , width :: Int
  , height :: Int
  , x :: Number
  , y :: Number
  }

foreign import dimensions :: Image -> Effect { width :: Int, height :: Int }
foreign import newImage :: { width :: Int, height :: Int } -> Effect Image
foreign import uploadImage :: Image -> Effect Image
foreign import clearImage :: Image -> Effect Unit
foreign import disposeImage :: Image -> Effect Unit
foreign import fillImage :: { target :: Image, color :: RGBA8 } -> Effect Unit
foreign import fillBackground :: { target :: Image, color :: Color } -> Effect Unit
foreign import drawImage :: { target :: Image, source :: Image, alpha :: Number } -> Effect Unit
foreign import compileShader :: String -> Effect Shader
foreign import drawShader :: ShaderDraw -> Effect Unit
foreign import compositeSource :: String
foreign import clipSource :: String
foreign import circleSource :: String
foreign import latticeSource :: String
foreign import blurSource :: Int -> String
foreign import newPath :: Effect NativePath
foreign import moveTo :: { path :: NativePath, point :: Point } -> Effect Unit
foreign import lineTo :: { path :: NativePath, point :: Point } -> Effect Unit
foreign import quadTo :: { path :: NativePath, control :: Point, point :: Point } -> Effect Unit
foreign import cubicTo :: { path :: NativePath, control1 :: Point, control2 :: Point, point :: Point } -> Effect Unit
foreign import closePath :: NativePath -> Effect Unit
foreign import fillPath :: { target :: Image, path :: NativePath, color :: RGBA8, evenOdd :: Boolean } -> Effect Unit
foreign import strokePath :: { target :: Image, path :: NativePath, color :: RGBA8, width :: Number, join :: Int, cap :: Int } -> Effect Unit
foreign import faceSource :: String -> Effect FaceSource
foreign import sameFaceSource :: FaceSource -> FaceSource -> Boolean
foreign import newFace :: { source :: FaceSource, size :: Number } -> Effect Face
foreign import faceMetrics :: Face -> Effect FaceMetrics
foreign import measureText :: { face :: Face, text :: String } -> Effect Number
foreign import drawText :: { target :: Image, face :: Face, text :: String, x :: Number, y :: Number, align :: Int, color :: RGBA8 } -> Effect Unit
