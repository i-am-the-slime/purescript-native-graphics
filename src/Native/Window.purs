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
import Effect (Effect)
import Native.Graphics (Bytes, Drawing)

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

foreign import configure :: Config -> Effect Unit
foreign import deviceScaleFactor :: Effect Number
foreign import platform :: String
foreign import run :: Options -> (Input -> Effect Output) -> Effect Unit
