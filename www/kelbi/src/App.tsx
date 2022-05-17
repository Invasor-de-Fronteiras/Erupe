import React from 'react';
import { SignInForm } from './layout/SignInForm';

import { AiFillGithub } from 'react-icons/ai';
import { GrConfigure } from 'react-icons/gr';
import { SiDiscord } from 'react-icons/si';
import { LogoIcon } from './components/LogoIcon';

import './styles/app.css';
import { Background } from './components/Background';
import { LauncherButtonGroup } from './components/buttons/LauncherButtonGroup';

function App() {
  return (
    <div id='main'>
      <div id='left-side'>
        <div id='left-side-header'>
          <LogoIcon />
        </div>
        <SignInForm />
        <div id='left-side-footer'>
          <GrConfigure className='icon' size={20} />
          <div>
            <SiDiscord className='icon discord-icon' size={20} />
            <AiFillGithub className='icon' size={20} />
          </div>
        </div>
      </div>
      <Background />
      <LauncherButtonGroup />
    </div>
  );
}

export default App;
