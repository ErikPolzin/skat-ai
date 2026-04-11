import React, { useState, useEffect } from 'react';
import { Box, Container, Typography, TextField, Button, Paper } from '@mui/material';

interface UsernameScreenProps {
  onSubmit: (username: string) => void;
}

export default function UsernameScreen({ onSubmit }: UsernameScreenProps) {
  const [username, setUsername] = useState('');

  useEffect(() => {
    // Load saved username from localStorage
    const savedUsername = localStorage.getItem('skat-username');
    if (savedUsername) {
      setUsername(savedUsername);
    }
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const name = username.trim() || 'Player';
    localStorage.setItem('skat-username', name);
    onSubmit(name);
  };

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        px: 2
      }}>
      <Container maxWidth="sm">
        <Paper elevation={3} sx={{ p: 5, textAlign: 'center' }}>
          <Typography variant="h3" component="h1" gutterBottom>
            Welcome to Skat
          </Typography>
          <Box component="form" onSubmit={handleSubmit} sx={{ mt: 3 }}>
            <TextField
              id="username"
              label="Enter your username"
              placeholder="Your name"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              fullWidth
              autoFocus
              sx={{ mb: 3 }}
            />
            <Button
              type="submit"
              variant="contained"
              color="primary"
              fullWidth
              size="large"
            >
              Continue
            </Button>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
