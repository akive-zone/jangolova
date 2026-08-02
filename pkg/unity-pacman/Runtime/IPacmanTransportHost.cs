using System;

namespace Jangolova.Pacman
{
    // A transport host only carries Pacman request/response envelopes. It does
    // not define semantic methods or own the Unity application lifecycle.
    public interface IPacmanTransportHost : IDisposable
    {
        void StartHost(PacmanBridge bridge);
    }
}
