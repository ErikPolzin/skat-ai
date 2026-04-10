import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface ProfileState {
  username: string | null;
  playerId: string | null;
}

interface ProfileActions {
  setUsername: (username: string) => void;
  setPlayerId: (playerId: string) => void;
  clearProfile: () => void;
}

type ProfileStore = ProfileState & ProfileActions;

export const useProfileStore = create<ProfileStore>()(
  persist(
    (set) => ({
      username: null,
      playerId: null, // Will be set by backend on first join
      setUsername: (username: string) => set({ username }),
      setPlayerId: (playerId: string) => set({ playerId }),
      clearProfile: () => set({ username: null, playerId: null }),
    }),
    {
      name: 'skat-profile',
    }
  )
);

// Selectors
export const selectUsername = (state: ProfileStore) => state.username;
export const selectPlayerId = (state: ProfileStore) => state.playerId;
export const selectSetUsername = (state: ProfileStore) => state.setUsername;
export const selectSetPlayerId = (state: ProfileStore) => state.setPlayerId;
export const selectClearProfile = (state: ProfileStore) => state.clearProfile;
