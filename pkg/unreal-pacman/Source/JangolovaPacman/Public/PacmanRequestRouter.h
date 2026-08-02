#pragma once

#include "CoreMinimal.h"

class UPacmanRegistryComponent;

// Parses and envelopes Pacman JSON messages. Semantic dispatch is always
// marshalled to the Unreal game thread; the caller may receive the reply on
// any thread used by its WebSocket implementation.
class JANGOLOVAPACMAN_API FPacmanRequestRouter final
    : public TSharedFromThis<FPacmanRequestRouter>
{
public:
    using FReply = TFunction<void(const FString&)>;

    explicit FPacmanRequestRouter(TWeakObjectPtr<UPacmanRegistryComponent> InRegistry);

    bool HandleText(const FString& Message, FReply Reply);
    void Stop();

private:
    TWeakObjectPtr<UPacmanRegistryComponent> Registry;
    TAtomic<bool> Stopped { false };
};
