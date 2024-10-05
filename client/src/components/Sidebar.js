import React from "react";
import {
  AiOutlineFileSearch,
  AiOutlineHome,
  AiOutlineSetting,
} from "react-icons/ai";
import { BiRocket } from "react-icons/bi";
import "./Sidebar.css";
import torrent from "../images/torrent.png";
import { sideBar } from "../constants";

const Sidebar = ({ setScreen, inputRef }) => {
  const navItems = [
    {
      icon: <AiOutlineHome />,
      label: "DASHBOARD",
      action: () => setScreen(sideBar.DASHBOARD),
    },
    {
      icon: <BiRocket />,
      label: "TORRENTS",
      action: () => setScreen(sideBar.TORRENTS),
    },
    {
      icon: <AiOutlineFileSearch />,
      label: "Search",
      action: () => inputRef.current.focus(),
    },
    {
      icon: <AiOutlineSetting />,
      label: "SETTINGS",
      action: () => console.log("Settings clicked"),
    },
  ];

  return (
    <aside className="sidebar">
      <div className="sidebar-logo">
        <img src={torrent} alt="Logo" />
      </div>
      <nav className="sidebar-nav">
        <ul>
          {navItems.map(({ icon, label, action }, index) => (
            <li key={index} onClick={action}>
              <div className="nav-item">
                {icon}
                <p>{label}</p>
              </div>
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  );
};

export default Sidebar;
