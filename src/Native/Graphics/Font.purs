module Native.Graphics.Font (registerFont, resolveFamily) where

import Prelude

import Control.Alt ((<|>))
import Control.Monad.Rec.Class (Step(..), tailRec, tailRecM)
import Data.Array as Array
import Data.Either (hush)
import Data.Maybe (Maybe(..), isJust, maybe)
import Data.Number as Number
import Data.String.CodeUnits as CodeUnits
import Data.String.Common as String
import Data.String.Pattern (Pattern(..))
import Data.Tuple (fst)
import Effect (Effect)
import Effect.Exception (throw)
import Effect.Ref (Ref)
import Effect.Ref as Ref
import Native.Graphics.Types (Bytes)
import Parsing (Parser, fail, runParser)
import Parsing.Combinators (lookAhead, optionMaybe, skipMany, skipMany1, try)
import Parsing.String (anyChar, char, eof, match, satisfy, string)

type Font = { font :: String, size :: Number, family :: String }
type Context = Ref Font
type Dimensions = { width :: Number, height :: Number }
type Canvas = { dimensions :: Ref Dimensions, context :: Context }
type Metrics = { width :: Number, ascent :: Number, descent :: Number }

type Callbacks =
  { createCanvas :: Number -> Number -> Effect Canvas
  , getContext2D :: Canvas -> Effect Context
  , setFont :: Context -> String -> Effect Unit
  , font :: Context -> Effect String
  , measureText :: Context -> String -> Effect { width :: Number }
  , measureTextInk :: Context -> String -> Effect Metrics
  , getCanvasWidth :: Canvas -> Effect Number
  , getCanvasHeight :: Canvas -> Effect Number
  , setCanvasWidth :: Canvas -> Number -> Effect Unit
  , setCanvasHeight :: Canvas -> Number -> Effect Unit
  , checkFont :: String -> Effect Boolean
  }

-- The native boundary installs these callbacks once. Each canvas owns its refs;
-- registering resources never creates a process-global PureScript context.
registerFont :: { name :: String, data :: Bytes, features :: String } -> Effect Unit
registerFont resource = do
  installCallbacks callbacks
  registerResource resource

callbacks :: Callbacks
callbacks =
  { createCanvas
  , getContext2D: pure <<< _.context
  , setFont
  , font: map _.font <<< Ref.read
  , measureText: \context content -> do
      metrics <- measureTextInk context content
      pure { width: metrics.width }
  , measureTextInk
  , getCanvasWidth: \canvas -> _.width <$> Ref.read canvas.dimensions
  , getCanvasHeight: \canvas -> _.height <$> Ref.read canvas.dimensions
  , setCanvasWidth: \canvas width -> Ref.modify_ (_ { width = width }) canvas.dimensions
  , setCanvasHeight: \canvas height -> Ref.modify_ (_ { height = height }) canvas.dimensions
  , checkFont
  }

-- Preserve the native shorthand grammar, including ASCII whitespace, optional
-- prefixes and line-height, and a family remainder without a final newline.
fontShorthand :: Parser String { size :: Number, family :: String }
fontShorthand = do
  parsed <- tailRecM step unit
  value <- maybe (fail "font size") pure (Number.fromString parsed.sizeText)
  pure { size: value * parsed.multiplier, family: parsed.family }
  where
  whitespace c = c == '\t' || c == '\n' || c == '\x000c' || c == '\r' || c == ' '
  space = satisfy whitespace
  nonspace = satisfy (not <<< whitespace)
  digit = satisfy \c -> c >= '0' && c <= '9'

  step _ =
    (Done <$> try sizeAndFamily) <|>
      (skipMany nonspace *> skipMany1 space $> Loop unit)

  sizeAndFamily = do
    sizeText <- fst <$> match
      (skipMany1 digit *> optionMaybe (char '.' *> skipMany1 digit))
    multiplier <- (string "px" $> 1.0) <|> (string "pt" $> (96.0 / 72.0))
    void $ optionMaybe (char '/' *> skipMany1 nonspace)
    void space
    -- The separator is greedy, but must leave a nonempty family remainder.
    skipMany (try (space <* lookAhead anyChar))
    family <- fst <$> match (skipMany1 (satisfy (_ /= '\n')))
    eof
    pure { sizeText, multiplier, family }

parseFont :: String -> Maybe Font
parseFont font = do
  parsed <- hush (runParser font fontShorthand)
  pure { font, size: parsed.size, family: parsed.family }

-- Match the existing family-list contract: split at commas, trim whitespace,
-- then surrounding quote characters, and choose the first registered resource.
familyName :: String -> String
familyName input = CodeUnits.take end unquoted
  where
  quote c = c == '"' || c == '\''
  unquoted = CodeUnits.dropWhile quote (String.trim input)
  end = tailRec
    ( \index -> case CodeUnits.charAt (index - 1) unquoted of
        Just c | quote c -> Loop (index - 1)
        _ -> Done index
    )
    (CodeUnits.length unquoted)

selectFamily :: String -> Effect (Maybe String)
selectFamily families = tailRecM step 0
  where
  names = String.split (Pattern ",") families
  step index = case Array.index names index of
    Nothing -> pure (Done Nothing)
    Just candidate -> do
      let name = familyName candidate
      registered <- isRegistered name
      pure (if registered then Done (Just name) else Loop (index + 1))

resolveFamily :: String -> Effect String
resolveFamily families = do
  selected <- selectFamily families
  case selected of
    Just name -> pure name
    Nothing -> throw ("native graphics: no registered face in " <> show families)

createCanvas :: Number -> Number -> Effect Canvas
createCanvas width height = do
  dimensions <- Ref.new { width, height }
  context <- Ref.new { font: "", size: 0.0, family: "" }
  pure { dimensions, context }

setFont :: Context -> String -> Effect Unit
setFont context shorthand = case parseFont shorthand of
  Nothing -> pure unit
  Just font -> Ref.write font context

measureTextInk :: Context -> String -> Effect Metrics
measureTextInk context text = do
  font <- Ref.read context
  name <- resolveFamily font.family
  measureNative { name, size: font.size, text }

checkFont :: String -> Effect Boolean
checkFont shorthand = case parseFont shorthand of
  Nothing -> pure false
  Just font -> isJust <$> selectFamily font.family

foreign import installCallbacks :: Callbacks -> Effect Unit
foreign import registerResource :: { name :: String, data :: Bytes, features :: String } -> Effect Unit
foreign import isRegistered :: String -> Effect Boolean
foreign import measureNative :: { name :: String, size :: Number, text :: String } -> Effect Metrics
