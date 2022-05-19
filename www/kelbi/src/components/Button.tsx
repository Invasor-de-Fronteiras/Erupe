import React from 'react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
  isLoading?: boolean;
}

export function Button({ isLoading, children, disabled, ...props }: ButtonProps) {
  return (
    <button {...props} disabled={disabled || isLoading}>
      {isLoading ? <span className='loading'>Carregando...</span> : children}
    </button>
  );
}
