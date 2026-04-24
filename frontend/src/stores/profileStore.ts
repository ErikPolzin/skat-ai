import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface ProfileState {
  username: string | null;
  playerId: string | null;
  profileIcon: string | null;
}

interface ProfileActions {
  setUsername: (username: string) => void;
  setPlayerId: (playerId: string) => void;
  setProfileIcon: (profileIcon: string) => void;
  clearProfile: () => void;
}

type ProfileStore = ProfileState & ProfileActions;

export const useProfileStore = create<ProfileStore>()(
  persist(
    (set) => ({
      username: null,
      playerId: null, // Will be set by backend on first join
      profileIcon: null,
      setUsername: (username: string) => set({ username }),
      setPlayerId: (playerId: string) => set({ playerId }),
      setProfileIcon: (profileIcon: string) => set({ profileIcon }),
      clearProfile: () => set({ username: null, playerId: null, profileIcon: null }),
    }),
    {
      name: 'skat-profile',
    }
  )
);

// Selectors
export const selectUsername = (state: ProfileStore) => state.username;
export const selectPlayerId = (state: ProfileStore) => state.playerId;
export const selectProfileIcon = (state: ProfileStore) => state.profileIcon;
export const selectSetUsername = (state: ProfileStore) => state.setUsername;
export const selectSetPlayerId = (state: ProfileStore) => state.setPlayerId;
export const selectSetProfileIcon = (state: ProfileStore) => state.setProfileIcon;
export const selectClearProfile = (state: ProfileStore) => state.clearProfile;
