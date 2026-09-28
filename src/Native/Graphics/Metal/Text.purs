module Native.Graphics.Metal.Text
  ( create
  , decodeAtlas
  , textGeometry
  , Geometry
  , Atlas
  , Font
  , Entry
  , ShapedText
  , Glyph
  , Resources
  , FontResource
  ) where

import Prelude hiding (top)

import Control.Monad.ST as ST
import Control.Monad.ST.Ref as STRef
import Data.Array as Array
import Data.Either (Either(..))
import Data.Enum (fromEnum)
import Data.Foldable (foldl)
import Data.Int as Int
import Data.Map (Map)
import Data.Map as Map
import Data.Maybe (Maybe(..), fromMaybe)
import Data.String.CodePoints as CodePoints
import Data.Traversable (traverse)
import Effect (Effect)
import Effect.Exception (throw)
import Effect.Ref as Ref
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Metal.Vertices (Vertices)
import Native.Graphics.Metal.Vertices as Vertices
import Native.Graphics.Types (TextAlign(..), TextBaseline(..), TextSpec, Transform)
import Yoga.JSON (readJSON)

type Bounds = { left :: Number, bottom :: Number, right :: Number, top :: Number }

type AtlasGlyph =
  { index :: Maybe Int
  , unicode :: Maybe Int
  , planeBounds :: Maybe Bounds
  , atlasBounds :: Maybe Bounds
  }

type AtlasMetadata =
  { atlas ::
      { distanceRange :: Number
      , size :: Number
      , width :: Int
      , height :: Int
      , yOrigin :: Maybe String
      }
  , variants :: Array { glyphs :: Array AtlasGlyph }
  }

type FontResource = { atlasVariant :: Int, unicodeKeys :: Boolean }

type Resources =
  { atlasJSON :: String
  , width :: Int
  , height :: Int
  , fonts :: Array FontResource
  }

type Entry =
  { px0 :: Number
  , py0 :: Number
  , px1 :: Number
  , py1 :: Number
  , u0 :: Number
  , v0 :: Number
  , u1 :: Number
  , v1 :: Number
  }

type Font = { unicodeKeys :: Boolean, glyphs :: Map Int (Maybe Entry) }

type Atlas = { distanceRange :: Number, size :: Number, fonts :: Array Font }

type Glyph =
  { glyphID :: Int
  , sourceIndex :: Int
  , xOffset :: Number
  , yOffset :: Number
  , advance :: Number
  }

type ShapedText =
  { glyphs :: Array Glyph
  , width :: Number
  , capHeight :: Number
  , ascent :: Number
  , descent :: Number
  , backingScale :: Number
  }

type Geometry = { vertices :: Vertices, screenPxRange :: Number }

-- | Resource loading is lazy because the renderer is created before Configure.
-- | A new native resource generation invalidates only the decoded atlas cache.
-- | Size is already in view points; only the text origin is transformed here.
create :: Effect (Transform -> Number -> TextSpec -> Effect Geometry)
create = do
  cache <- Ref.new Nothing
  pure \transform inheritedAlpha spec ->
    if spec.text == "" || spec.size <= 0.0 || spec.fontID < 0 then
      pure emptyGeometry
    else do
      version <- resourceVersion
      previous <- Ref.read cache
      atlas <- case previous of
        Just stored | stored.version == version -> pure stored.atlas
        _ -> do
          resources <- readResources
          decoded <- case decodeAtlas resources of
            Left message -> throw message
            Right value -> pure value
          Ref.write (Just { version, atlas: decoded }) cache
          pure decoded
      case Array.index atlas.fonts spec.fontID of
        Nothing -> pure emptyGeometry
        Just font -> do
          shaped <- shapeText spec.fontID spec.text spec.size
          pure (textGeometry transform inheritedAlpha spec atlas font shaped)

emptyGeometry :: Geometry
emptyGeometry = { vertices: Vertices.empty, screenPxRange: 0.0 }

decodeAtlas :: Resources -> Either String Atlas
decodeAtlas resources
  | Array.null resources.fonts = Right { distanceRange: 0.0, size: 1.0, fonts: [] }
  | otherwise = case readJSON @AtlasMetadata resources.atlasJSON of
      Left errors -> Left ("MSDF metadata: " <> show errors)
      Right meta
        | meta.atlas.width <= 0 || meta.atlas.height <= 0 || meta.atlas.size <= 0.0 || meta.atlas.distanceRange <= 0.0 ->
            Left "MSDF metadata requires positive dimensions, size and distance range"
        | meta.atlas.width /= resources.width || meta.atlas.height /= resources.height ->
            Left "MSDF image dimensions do not match metadata"
        | otherwise -> do
            fonts <- traverse (decodeFont meta) resources.fonts
            pure { distanceRange: meta.atlas.distanceRange, size: meta.atlas.size, fonts }

decodeFont :: AtlasMetadata -> FontResource -> Either String Font
decodeFont meta resource = case Array.index meta.variants resource.atlasVariant of
  Nothing -> Left ("MSDF atlas variant " <> show resource.atlasVariant <> " is out of range")
  Just variant -> Right
    { unicodeKeys: resource.unicodeKeys
    , glyphs: foldl insertGlyph Map.empty variant.glyphs
    }
  where
  insertGlyph glyphs glyph =
    let
      key = fromMaybe 0 (if resource.unicodeKeys then glyph.unicode else glyph.index)
      entry = do
        plane <- glyph.planeBounds
        bounds <- glyph.atlasBounds
        let
          width = Int.toNumber meta.atlas.width
          height = Int.toNumber meta.atlas.height
          topOrigin = meta.atlas.yOrigin == Just "top"
        pure
          { px0: plane.left
          , py0: if topOrigin then plane.top else -plane.top
          , px1: plane.right
          , py1: if topOrigin then plane.bottom else -plane.bottom
          , u0: bounds.left / width
          , v0: if topOrigin then bounds.top / height else (height - bounds.top) / height
          , u1: bounds.right / width
          , v1: if topOrigin then bounds.bottom / height else (height - bounds.bottom) / height
          }
    in
      Map.insert key entry glyphs

textGeometry :: Transform -> Number -> TextSpec -> Atlas -> Font -> ShapedText -> Geometry
textGeometry transform inheritedAlpha spec atlas font shaped =
  let
    origin = Geometry.apply transform spec.x spec.y
    baseline = origin.y + case spec.baseline of
      Alphabetic -> 0.0
      Middle -> shaped.capHeight / 2.0
      Top -> shaped.ascent
      Bottom -> -shaped.descent
    bold = spec.size * spec.boldOffset
    width = shaped.width + bold
    startX = origin.x - case spec.align of
      AlignLeft -> 0.0
      AlignCenter -> width / 2.0
      AlignRight -> width
    backingScale = if shaped.backingScale <= 0.0 then 1.0 else shaped.backingScale
    codePoints = if font.unicodeKeys then map fromEnum (CodePoints.toCodePointArray spec.text) else []
    alpha = spec.color.alpha * inheritedAlpha
    vertices = ST.run do
      buffer <- Vertices.new (Array.length shaped.glyphs * 48 * (if bold == 0.0 then 1 else 2))
      let
        appendRun runX = do
          cursor <- STRef.new runX
          ST.foreach shaped.glyphs \glyph -> do
            x <- STRef.read cursor
            let
              entry = do
                key <- if font.unicodeKeys then Array.index codePoints glyph.sourceIndex else Just glyph.glyphID
                join (Map.lookup key font.glyphs)
            case entry of
              Nothing -> pure unit
              Just bounds -> do
                let
                  gx = x + glyph.xOffset
                  gy = baseline - glyph.yOffset
                  x0 = gx + bounds.px0 * spec.size
                  y0 = gy + bounds.py0 * spec.size
                  x1 = gx + bounds.px1 * spec.size
                  y1 = gy + bounds.py1 * spec.size
                  r = spec.color.red
                  g = spec.color.green
                  b = spec.color.blue
                Vertices.appendGlyphQuad buffer
                  [ x0
                  , y0
                  , x1
                  , y1
                  , bounds.u0
                  , bounds.v0
                  , bounds.u1
                  , bounds.v1
                  , r
                  , g
                  , b
                  , alpha
                  ]
            void $ STRef.modify (_ + glyph.advance) cursor
      appendRun startX
      when (bold /= 0.0) (appendRun (startX + bold))
      Vertices.freeze buffer
  in
    { vertices
    , screenPxRange: spec.size * backingScale * atlas.distanceRange / atlas.size
    }

foreign import resourceVersion :: Effect Int
foreign import readResources :: Effect Resources
foreign import shapeText :: Int -> String -> Number -> Effect ShapedText
