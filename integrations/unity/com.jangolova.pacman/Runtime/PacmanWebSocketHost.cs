using System;
using System.Threading;
using UnityEngine;

namespace Jangolova.Pacman
{
    [DisallowMultipleComponent]
    public sealed class PacmanWebSocketHost : MonoBehaviour, IPacmanTransportHost
    {
        [SerializeField] private string listenerPrefix;
        private PacmanWebSocketServer server;

        public void StartHost(PacmanBridge bridge)
        {
            if (server != null) throw new InvalidOperationException("Pacman WebSocket host is already running.");
            string prefix = string.IsNullOrWhiteSpace(listenerPrefix)
                ? Environment.GetEnvironmentVariable(PacmanProtocol.PrefixEnvironmentVariable)
                : listenerPrefix;
            string token = Environment.GetEnvironmentVariable(PacmanProtocol.TokenEnvironmentVariable);
            if (string.IsNullOrWhiteSpace(prefix))
                throw new InvalidOperationException("Pacman WebSocket listener prefix is required.");
            server = new PacmanWebSocketServer(bridge, prefix, token, SynchronizationContext.Current);
            server.Start();
        }

        public void Dispose()
        {
            if (server == null) return;
            server.Dispose();
            server = null;
        }

        private void OnDestroy() { Dispose(); }
    }
}
