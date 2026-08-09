import LoginScreen from "./LoginScreen";

interface SignupScreenProps {
  isSubmitting?: boolean;
  error?: string | null;
  onSwitchMode?: () => void;
  onSubmit: (username: string, password: string) => Promise<void>;
}

export default function SignupScreen(props: SignupScreenProps) {
  return <LoginScreen {...props} mode="sign-up" />;
}
