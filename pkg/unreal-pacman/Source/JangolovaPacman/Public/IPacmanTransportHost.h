#pragma once

#include "CoreMinimal.h"

class UPacmanRegistryComponent;

// A concrete Unreal WebSocket implementation supplies this interface. The
// Pacman host owns only the connection and callbacks, never the application.
class JANGOLOVAPACMAN_API IPacmanWebSocketConnection
{
public:
    using FTextHandler = TFunction<void(const FString&)>;

    virtual ~IPacmanWebSocketConnection() = default;
    virtual FString AuthorizationHeader() const = 0;
    virtual void SetTextHandler(FTextHandler Handler) = 0;
    virtual void SendText(const FString& Message) = 0;
    virtual void Close() = 0;
};

// Transport implementations authenticate and frame Pacman requests. They do
// not own the Unreal application, World, renderer, or target lifecycle.
class JANGOLOVAPACMAN_API IPacmanTransportHost
{
public:
    virtual ~IPacmanTransportHost() = default;
    virtual void StartHost(TWeakObjectPtr<UPacmanRegistryComponent> Registry) = 0;
    virtual void StopHost() = 0;
};
