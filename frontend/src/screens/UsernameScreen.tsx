import React, { useState, useEffect } from 'react';
import './UsernameScreen.css';

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
    <div className="screen">
      <div className="container username-container">
        <h1>Welcome to Skat</h1>
        <form onSubmit={handleSubmit} className="username-form">
          <div className="input-group">
            <label htmlFor="username">Enter your username</label>
            <input
              id="username"
              type="text"
              placeholder="Your name"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
            />
          </div>
          <button type="submit" className="btn btn-primary">
            Continue
          </button>
        </form>
      </div>
    </div>
  );
}
