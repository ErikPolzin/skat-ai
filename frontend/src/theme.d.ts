import "@mui/material/styles";

declare module "@mui/material/styles" {
  interface Theme {
    themeRedColor: string;
    themeBlackColor: string;
  }

  interface ThemeOptions {
    themeRedColor?: string;
    themeBlackColor?: string;
  }
}
