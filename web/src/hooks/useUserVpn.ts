import { useCallback, useState } from 'react';
import { api } from '../api';
import { downloadConfFile } from '../utils/format';
import type { PeerProvisionResponse, WGNetwork, WGPeer } from '../types';

interface UseUserVpnParams {
  onError: (msg: string | null) => void;
  onNotice: (msg: string | null) => void;
  onAdminRefresh?: () => Promise<void>;
}

export function useUserVpn({ onError, onNotice, onAdminRefresh }: UseUserVpnParams) {
  const [networks, setNetworks] = useState<WGNetwork[]>([]);
  const [myPeers, setMyPeers] = useState<WGPeer[]>([]);
  const [quota, setQuota] = useState<{ used: number; limit: number }>({ used: 0, limit: 5 });

  // Create Peer Modal state
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [selectedNetId, setSelectedNetId] = useState<number>(0);
  const [peerName, setPeerName] = useState('');
  const [keyModeChoice, setKeyModeChoice] = useState<'server' | 'custom'>('server');
  const [customPublicKey, setCustomPublicKey] = useState('');
  const [submittingPeer, setSubmittingPeer] = useState(false);

  // Regenerate Keys Modal state
  const [regenTarget, setRegenTarget] = useState<WGPeer | null>(null);
  const [regenModeChoice, setRegenModeChoice] = useState<'server' | 'custom'>('server');
  const [regenCustomPubKey, setRegenCustomPubKey] = useState('');
  const [submittingRegen, setSubmittingRegen] = useState(false);

  // Provisioned Result Modal (QR Code + .conf)
  const [provisionResult, setProvisionResult] = useState<PeerProvisionResponse | null>(null);

  const refreshUserDashboard = useCallback(async () => {
    const [netRes, peerRes] = await Promise.all([api.listNetworks(), api.listMyPeers()]);
    const netList = netRes.networks || [];
    setNetworks(netList);
    setMyPeers(peerRes.peers || []);
    setQuota(peerRes.quota || { used: 0, limit: 5 });
    setSelectedNetId((prev) => (prev === 0 && netList.length > 0 ? netList[0].id : prev));
  }, []);

  const openCreatePeerModal = useCallback(
    (networkId?: number) => {
      onError(null);
      if (networkId) {
        setSelectedNetId(networkId);
      } else if (networks.length > 0 && !selectedNetId) {
        setSelectedNetId(networks[0].id);
      }
      setPeerName('');
      setKeyModeChoice('server');
      setCustomPublicKey('');
      setCreateModalOpen(true);
    },
    [networks, onError, selectedNetId]
  );

  const submitCreatePeer = useCallback(async () => {
    onError(null);
    setSubmittingPeer(true);
    try {
      const res = await api.createPeer({
        network_id: selectedNetId,
        name: peerName,
        public_key: keyModeChoice === 'custom' ? customPublicKey : '',
      });
      setCreateModalOpen(false);
      setProvisionResult(res);
      await refreshUserDashboard();
      if (onAdminRefresh) {
        await onAdminRefresh();
      }
    } catch (err: any) {
      onError(err.message);
    } finally {
      setSubmittingPeer(false);
    }
  }, [
    customPublicKey,
    keyModeChoice,
    onAdminRefresh,
    onError,
    peerName,
    refreshUserDashboard,
    selectedNetId,
  ]);

  const openRegenerateModal = useCallback(
    (peer: WGPeer) => {
      onError(null);
      setRegenTarget(peer);
      setRegenModeChoice('server');
      setRegenCustomPubKey('');
    },
    [onError]
  );

  const submitRegenerateKeys = useCallback(async () => {
    if (!regenTarget) return;
    onError(null);
    setSubmittingRegen(true);
    try {
      const res = await api.regeneratePeerKeys(
        regenTarget.id,
        regenModeChoice === 'custom' ? regenCustomPubKey : ''
      );
      setRegenTarget(null);
      setProvisionResult(res);
      onNotice(
        `Keys regenerated in-place for "${res.peer.name}" — IP ${res.peer.allocated_ip} preserved.`
      );
      await refreshUserDashboard();
      if (onAdminRefresh) {
        await onAdminRefresh();
      }
    } catch (err: any) {
      onError(err.message);
    } finally {
      setSubmittingRegen(false);
    }
  }, [
    onAdminRefresh,
    onError,
    onNotice,
    refreshUserDashboard,
    regenCustomPubKey,
    regenModeChoice,
    regenTarget,
  ]);

  const downloadTemplate = useCallback(
    async (peer: WGPeer) => {
      try {
        const res = await api.getPeerConfigTemplate(peer.id);
        const safeName = peer.name.toLowerCase().replace(/[^a-z0-9-_]/g, '-') || 'wg-peer';
        downloadConfFile(`${safeName}-${peer.allocated_ip}.conf`, res.config_text);
      } catch (err: any) {
        onError(err.message);
      }
    },
    [onError]
  );

  const deletePeer = useCallback(
    async (peer: WGPeer) => {
      if (
        !window.confirm(
          `Delete VPN connection "${peer.name}" (${peer.allocated_ip})?\nIts IP address (${peer.allocated_ip}) will be released for reuse.`
        )
      ) {
        return;
      }
      onError(null);
      try {
        const res = await api.deletePeer(peer.id);
        onNotice(
          `Removed "${peer.name}" from RouterOS. IP ${res.released_ip} is now free for reuse.`
        );
        await refreshUserDashboard();
        if (onAdminRefresh) {
          await onAdminRefresh();
        }
      } catch (err: any) {
        onError(err.message);
      }
    },
    [onAdminRefresh, onError, onNotice, refreshUserDashboard]
  );

  return {
    networks,
    myPeers,
    quota,
    quotaReached: quota.used >= quota.limit,
    refreshUserDashboard,
    // Create modal
    createModalOpen,
    setCreateModalOpen,
    selectedNetId,
    setSelectedNetId,
    peerName,
    setPeerName,
    keyModeChoice,
    setKeyModeChoice,
    customPublicKey,
    setCustomPublicKey,
    submittingPeer,
    openCreatePeerModal,
    submitCreatePeer,
    // Regenerate modal
    regenTarget,
    setRegenTarget,
    regenModeChoice,
    setRegenModeChoice,
    regenCustomPubKey,
    setRegenCustomPubKey,
    submittingRegen,
    openRegenerateModal,
    submitRegenerateKeys,
    // Result modal
    provisionResult,
    setProvisionResult,
    // Actions
    downloadTemplate,
    deletePeer,
  };
}
