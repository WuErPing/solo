import { useCallback, useMemo, useState } from "react";
import { ActivityIndicator, Pressable, Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Check, ShieldAlert } from "lucide-react-native";
import {
  PairWebRequestExpiredError,
  sendPairWebOffer,
} from "@server/relay/pair-web-channel";
import {
  isPairWebRequestExpired,
  publicKeyFingerprint,
  type PairWebRequest,
} from "@server/shared/pair-web-offer";
import { useHosts } from "@/runtime/host-runtime";
import { useNowMs } from "@/hooks/use-now";
import {
  buildConnectionOfferFromRelayConnection,
  formatPairWebCountdown,
  selectPairWebEligibleHosts,
  type PairWebEligibleHost,
} from "@/utils/pair-web";
import { Button } from "@/components/ui/button";
import { Fonts } from "@/constants/theme";

const styles = StyleSheet.create((theme) => ({
  container: {
    gap: theme.spacing[4],
  },
  warningCard: {
    flexDirection: "row",
    gap: theme.spacing[3],
    padding: theme.spacing[4],
    borderRadius: theme.borderRadius.lg,
    backgroundColor: theme.colors.surface2,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  warningText: {
    flex: 1,
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  metaLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  fingerprint: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontFamily: Fonts.mono,
  },
  countdown: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  countdownExpired: {
    color: theme.colors.destructive,
  },
  pickerLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
  },
  hostRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[3],
    padding: theme.spacing[4],
    borderRadius: theme.borderRadius.lg,
    backgroundColor: theme.colors.surface2,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  hostRowSelected: {
    borderColor: theme.colors.accent,
  },
  hostRadio: {
    width: 16,
    height: 16,
    borderRadius: 8,
    borderWidth: 2,
    borderColor: theme.colors.border,
    alignItems: "center",
    justifyContent: "center",
  },
  hostRadioSelected: {
    borderColor: theme.colors.accent,
    backgroundColor: theme.colors.accent,
  },
  hostBody: {
    flex: 1,
  },
  hostLabel: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.medium,
  },
  hostEndpoint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  error: {
    color: theme.colors.destructive,
    fontSize: theme.fontSize.sm,
  },
  actions: {
    flexDirection: "row",
    gap: theme.spacing[3],
    marginTop: theme.spacing[2],
  },
  centered: {
    alignItems: "center",
    gap: theme.spacing[4],
    paddingVertical: theme.spacing[8],
  },
  statusText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
}));

const FLEX_ONE_STYLE = { flex: 1 } as const;

type PanelPhase = "confirm" | "sending" | "sent" | "error";

export interface PairWebConfirmPanelProps {
  request: PairWebRequest;
  /** Back to the camera without approving. */
  onCancel: () => void;
  /** Leave the whole pairing flow after a successful handoff. */
  onDone: () => void;
}

export function PairWebConfirmPanel({ request, onCancel, onDone }: PairWebConfirmPanelProps) {
  const { theme } = useUnistyles();
  const hosts = useHosts();
  const eligible = useMemo(() => selectPairWebEligibleHosts(hosts), [hosts]);
  const [selectedServerId, setSelectedServerId] = useState<string | null>(
    eligible[0]?.host.serverId ?? null,
  );
  const [phase, setPhase] = useState<PanelPhase>("confirm");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const nowMs = useNowMs(1000, phase === "confirm");

  const expired = isPairWebRequestExpired(request, nowMs);
  const secondsLeft = Math.max(0, Math.ceil((request.exp * 1000 - nowMs) / 1000));
  const fingerprint = useMemo(() => publicKeyFingerprint(request.webPublicKeyB64), [request]);

  const selected =
    eligible.find((entry) => entry.host.serverId === selectedServerId) ?? eligible[0] ?? null;

  const handleApprove = useCallback(async () => {
    if (!selected || phase === "sending") {
      return;
    }
    setPhase("sending");
    setErrorMessage(null);
    try {
      const offer = buildConnectionOfferFromRelayConnection({
        serverId: selected.host.serverId,
        connection: selected.connection,
      });
      await sendPairWebOffer({ request, offer });
      setPhase("sent");
    } catch (error) {
      if (error instanceof PairWebRequestExpiredError) {
        setErrorMessage(
          "This QR code has expired. Ask the browser to generate a new one, then scan again.",
        );
      } else {
        setErrorMessage(error instanceof Error ? error.message : "Failed to send the credential.");
      }
      setPhase("error");
    }
  }, [phase, request, selected]);

  const handleApprovePress = useCallback(() => {
    void handleApprove();
  }, [handleApprove]);

  const handleTryAgain = useCallback(() => {
    setPhase("confirm");
    setErrorMessage(null);
  }, []);

  if (phase === "sending" || phase === "sent") {
    return (
      <View style={styles.centered} testID="pair-web-confirm-status">
        {phase === "sending" ? (
          <ActivityIndicator size="small" />
        ) : (
          <Check size={28} color={theme.colors.accent} />
        )}
        <Text style={styles.statusText}>
          {phase === "sending"
            ? "Sending the credential to your browser…"
            : "Approved. You can return to the browser."}
        </Text>
        {phase === "sent" ? (
          <Button
            variant="default"
            size="md"
            onPress={onDone}
            testID="pair-web-confirm-done"
            style={FLEX_ONE_STYLE}
          >
            Done
          </Button>
        ) : null}
      </View>
    );
  }

  if (eligible.length === 0) {
    return (
      <View style={styles.centered} testID="pair-web-confirm-empty">
        <Text style={styles.statusText}>
          This phone has no relay-paired computer to share. Pair a computer first.
        </Text>
        <Button variant="secondary" size="md" onPress={onCancel} style={FLEX_ONE_STYLE}>
          Back
        </Button>
      </View>
    );
  }

  return (
    <View style={styles.container} testID="pair-web-confirm-panel">
      <View style={styles.warningCard}>
        <ShieldAlert size={18} color={theme.colors.destructive} />
        <Text style={styles.warningText}>
          Only approve if you are signing in to Solo Web in your own browser right now. Approving
          hands this computer&apos;s access credentials to that browser tab.
        </Text>
      </View>

      <View>
        <Text style={styles.metaLabel}>Browser key fingerprint</Text>
        <Text style={styles.fingerprint} testID="pair-web-confirm-fingerprint">
          {fingerprint}
        </Text>
      </View>

      <Text
        style={[styles.countdown, expired ? styles.countdownExpired : null]}
        testID="pair-web-confirm-countdown"
      >
        {expired
          ? "This QR code has expired — ask the browser to generate a new one."
          : `Request expires in ${formatPairWebCountdown(secondsLeft)}`}
      </Text>

      {eligible.length > 1 ? (
        <View style={styles.container}>
          <Text style={styles.pickerLabel}>Share access to</Text>
          {eligible.map((entry) => (
            <HostPickerRow
              key={entry.host.serverId}
              entry={entry}
              selected={entry.host.serverId === selected?.host.serverId}
              onSelect={setSelectedServerId}
            />
          ))}
        </View>
      ) : null}

      {phase === "error" && errorMessage ? <Text style={styles.error}>{errorMessage}</Text> : null}

      <View style={styles.actions}>
        <Button
          style={FLEX_ONE_STYLE}
          variant="secondary"
          onPress={phase === "error" ? handleTryAgain : onCancel}
          testID="pair-web-confirm-cancel"
        >
          {phase === "error" ? "Try again" : "Cancel"}
        </Button>
        <Button
          style={FLEX_ONE_STYLE}
          variant="default"
          onPress={handleApprovePress}
          disabled={expired}
          testID="pair-web-confirm-approve"
        >
          Approve &amp; send
        </Button>
      </View>
    </View>
  );
}

function HostPickerRow({
  entry,
  selected,
  onSelect,
}: {
  entry: PairWebEligibleHost;
  selected: boolean;
  onSelect: (serverId: string) => void;
}) {
  const rowStyle = useMemo(
    () => [styles.hostRow, selected ? styles.hostRowSelected : null],
    [selected],
  );
  const radioStyle = useMemo(
    () => [styles.hostRadio, selected ? styles.hostRadioSelected : null],
    [selected],
  );
  const handlePress = useCallback(() => onSelect(entry.host.serverId), [entry.host.serverId, onSelect]);
  return (
    <Pressable
      style={rowStyle}
      onPress={handlePress}
      accessibilityRole="button"
      accessibilityLabel={`Share access to ${entry.host.label}`}
      testID={`pair-web-host-${entry.host.serverId}`}
    >
      <View style={radioStyle} />
      <View style={styles.hostBody}>
        <Text style={styles.hostLabel}>{entry.host.label}</Text>
        <Text style={styles.hostEndpoint}>{entry.connection.relayEndpoint}</Text>
      </View>
    </Pressable>
  );
}
