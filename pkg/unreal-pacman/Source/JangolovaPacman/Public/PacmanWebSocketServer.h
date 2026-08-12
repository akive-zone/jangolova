#pragma once

#include "CoreMinimal.h"
#include "IPacmanTransportHost.h"
#include "UObject/WeakObjectPtr.h"

class UPacmanRegistryComponent;
class IWebSocketServer;
class FPacmanWebSocketHost;
class IWebSocketClientConnection;

class JANGOLOVAPACMAN_API FPacmanUnrealWebSocketConnection final : public IPacmanWebSocketConnection
{
public:
    explicit FPacmanUnrealWebSocketConnection(const TSharedRef<IWebSocketClientConnection>& InConnection);
    virtual FString AuthorizationHeader() const override;
    virtual void SetTextHandler(FTextHandler InHandler) override;
    virtual void SendText(const FString& Message) override;
    virtual void Close() override;
    void Deliver(const FString& Message);

private:
    TSharedRef<IWebSocketClientConnection> Connection;
    FTextHandler Handler;
};

// UE 5.8's built-in WebSocketServer module provides the HTTP Upgrade binding.
// This adapter owns only the listener and connection wrappers; Unreal still
// owns the application and world lifecycle.
class JANGOLOVAPACMAN_API FPacmanWebSocketServer final
{
public:
    explicit FPacmanWebSocketServer(FString InBearerToken);
    ~FPacmanWebSocketServer();

    bool Start(uint16 Port, TWeakObjectPtr<UPacmanRegistryComponent> Registry);
    void Stop();
    bool IsListening() const;

private:
    FString BearerToken;
    TSharedPtr<IWebSocketServer> Server;
    TUniquePtr<FPacmanWebSocketHost> Host;
    TMap<IWebSocketClientConnection*, TSharedPtr<FPacmanUnrealWebSocketConnection>> Connections;
    bool Listening = false;
};
