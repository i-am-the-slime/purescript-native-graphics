module Native.Graphics
  ( Drawing
  , Image
  , Bytes
  , Command
  , Layer
  , Composite
  , makeDrawing
  , rasterize
  , capture
  , encodePng
  , encodePngBytes
  , bytesFromBase64
  , registerFont
  ) where

import Prelude
import Effect (Effect)

type Command =
  { kind :: Int
  , args :: Array Number
  , path :: Array Number
  , text :: String
  , font :: String
  , fontID :: Int
  }

type Layer = { global :: Boolean }

type Composite =
  { source :: Int
  , mask :: Int
  , invertMask :: Boolean
  , blend :: Int
  }

foreign import data Drawing :: Type
foreign import data Image :: Type
foreign import data Bytes :: Type

foreign import makeDrawing
  :: { commands :: Array Command
     , layers :: Array Layer
     , composite :: Array Composite
     , clear :: Array Number
     }
  -> Drawing

foreign import rasterize :: Int -> Int -> Drawing -> Effect Image
foreign import capture :: Int -> Int -> Drawing -> Effect Image
foreign import encodePng :: Image -> Effect (Array Int)
foreign import encodePngBytes :: Image -> Effect Bytes
foreign import bytesFromBase64 :: String -> Bytes
foreign import registerFont :: { name :: String, data :: Bytes, features :: String } -> Effect Unit
