module Native.Graphics.Metal.Primitives where

import Prelude
import Effect (Effect)
import Native.Graphics.Metal.Vertices (Vertices)

foreign import data Target :: Type
foreign import data Buffer :: Type
foreign import data Submission :: Type

foreign import beginFrame :: Effect Boolean
foreign import endFrame :: Effect Submission
foreign import awaitSubmission :: Submission -> Effect Unit
foreign import releaseSubmission :: Submission -> Effect Unit
foreign import submissionTiming :: Submission -> Effect { completed :: Boolean, gpuMilliseconds :: Number }
foreign import backingScale :: Effect Number
foreign import onClose :: Effect Unit -> Effect Unit
foreign import newTarget :: { width :: Int, height :: Int, multisample :: Boolean } -> Effect Target
foreign import releaseTarget :: Target -> Effect Unit
foreign import newBuffer :: Int -> Effect Buffer
foreign import releaseBuffer :: Buffer -> Effect Unit
foreign import beginPass :: { target :: Target, clear :: Boolean, color :: Array Number } -> Effect Unit
foreign import endPass :: Effect Unit
foreign import compose :: { sources :: Array Target, ops :: Array Int, destination :: Target, drawable :: Boolean } -> Effect Unit
foreign import filter :: { source :: Target, destination :: Target, uniform :: Array Number } -> Effect Unit
foreign import blit :: Target -> Effect Unit
foreign import stencil :: { state :: Int, depth :: Int, reference :: Int } -> Effect Unit
foreign import draw :: { buffer :: Buffer, offset :: Int, vertices :: Vertices, pipeline :: Int, stride :: Int, uniform :: Array Number } -> Effect Unit
