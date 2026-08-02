#pragma once

#include "CoreMinimal.h"

class UPacmanRegistryComponent;

// Transport implementations authenticate and frame Pacman requests. They do
// not own the Unreal application, World, renderer, or target lifecycle.
class JANGOLOVAPACMAN_API IPacmanTransportHost
{
public:
    virtual ~IPacmanTransportHost() = default;
    virtual void StartHost(TWeakObjectPtr<UPacmanRegistryComponent> Registry) = 0;
    virtual void StopHost() = 0;
};
