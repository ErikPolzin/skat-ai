import { create } from "zustand";
import { persist } from "zustand/middleware";
import { type PlayerRating } from "../api/games";

interface ProfileState {
  username: string | null;
  password: string | null;
  playerId: string | null;
  profileIcon: string | null;
  rating: PlayerRating | null;
}

interface ProfileActions {
  setUsername: (username: string) => void;
  setPassword: (password: string) => void;
  setPlayerId: (playerId: string) => void;
  setProfileIcon: (profileIcon: string) => void;
  setRating: (rating: PlayerRating) => void;
  clearProfile: () => void;
}

type ProfileStore = ProfileState & ProfileActions;

export const useProfileStore = create<ProfileStore>()(
  persist(
    (set) => ({
      username: null,
      password: null,
      playerId: null, // Will be set by backend on first join
      profileIcon: null,
      rating: null,
      setUsername: (username: string) => set({ username }),
      setPassword: (password: string) => set({ password }),
      setPlayerId: (playerId: string) => set({ playerId }),
      setProfileIcon: (profileIcon: string) => set({ profileIcon }),
      clearProfile: () =>
        set({ username: null, password: null, playerId: null, profileIcon: null }),
      setRating: (rating: PlayerRating) => set({ rating }),
    }),
    {
      name: "skat-profile",
    },
  ),
);

// Selectors
export const selectUsername = (state: ProfileStore) => state.username;
export const selectPassword = (state: ProfileStore) => state.password;
export const selectPlayerId = (state: ProfileStore) => state.playerId;
export const selectProfileIcon = (state: ProfileStore) => state.profileIcon;
export const selectRating = (state: ProfileStore) => state.rating;
// Setters
export const selectSetUsername = (state: ProfileStore) => state.setUsername;
export const selectSetPassword = (state: ProfileStore) => state.setPassword;
export const selectSetPlayerId = (state: ProfileStore) => state.setPlayerId;
export const selectSetProfileIcon = (state: ProfileStore) =>
  state.setProfileIcon;
export const selectClearProfile = (state: ProfileStore) => state.clearProfile;
export const selectSetRating = (state: ProfileStore) => state.setRating;
