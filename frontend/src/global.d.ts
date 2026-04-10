declare module '*.css' {
  const content: Record<string, string>;
  export default content;
}

declare namespace NodeJS {
  interface ProcessEnv {
    REACT_APP_API_URL?: string;
    REACT_APP_WS_URL?: string;
  }
}
