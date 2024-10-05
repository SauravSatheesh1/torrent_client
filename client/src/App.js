import React, { useEffect, useRef, useState } from "react";
import Sidebar from "./components/Sidebar";
import TorrentsList from "./components/TorrentsList";
import DragDrop from "./components/FileUpload";
import "./App.css";
import { BiSearch } from "react-icons/bi";
import axios from "axios";
import { sideBar } from "./constants";
import { PiDownloadSimple } from "react-icons/pi";
import styled from "@emotion/styled";

const AppContainer = styled.div`
  font-size: 16px;
  display: flex;
  height: 100vh;
  background-color: #1c1c1c;
  color: white;
  font-family: "FuturaFont", sans-serif;
`;

const MainContent = styled.div`
  width: 100%;
  margin: 0 20px 20px 0px;
  display: flex;
  flex-direction: column;
  background-color: #1c1c1c;
  align-items: center;
  gap: 8px;
`;

const SearchBarContainer = styled.div`
  width: 100%;
  padding-top: 10px;
  display: flex;
  justify-content: flex-start;
`;

const SearchBar = styled.div`
  width: 75%;
  display: flex;
  position: relative;
  align-items: center;
`;

const Input = styled.input`
  height: 40px;
  width: 100%;
  border-radius: 40px;
  border: none;
  padding-left: 25px;
  font-size: 12px;
  background-color: #2c2c3b;
  color: white;

  &:focus {
    outline: none; /* Ensure no outline on focus */
    border: none; /* Ensure no border on focus */
  }
`;

const TorrentContent = styled.div`
  width: 100%;
  padding: 20px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  background-color: #2c2c2e;
  align-items: center;
  gap: 8px;
  height: 100%;
`;

const UploadSection = styled.div`
  display: flex;
  gap: 16px;
`;

const DownloadBox = styled.div`
  height: 200px;
  width: 240px;
  background: #a6c0f1;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 20px;
`;

const IconContainer = styled.div`
  width: 80px;
  height: 80px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #abd5c2;
`;

const InfoContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: #000;
`;

const InfoText = styled.span`
  font-size: ${(props) => (props.size === "large" ? "20px" : "10px")};
  font-weight: ${(props) => (props.size === "large" ? "bold" : "normal")};
`;

function App() {
  const inputRef = useRef(null);
  const [screen, setScreen] = useState(sideBar.DASHBOARD);
  const [totalDownloaded, setTotalDownloaded] = useState("0 Bytes");
  const [files, setFiles] = useState({});
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  // Handle search input changes
  const handleSearchChange = (event) => {
    setSearchQuery(event.target.value);
  };

  // Filter files based on search query
  const filteredFiles = Object.entries(files).reduce(
    (acc, [fileName, fileData]) => {
      if (fileName.toLowerCase().includes(searchQuery.toLowerCase())) {
        acc[fileName] = fileData;
      }
      return acc;
    },
    {}
  );

  const handleFileSelect = async (e) => {
    const file = e;
    if (!file) return;

    const formData = new FormData();
    formData.append("torrentFile", file);

    setLoading(true);

    try {
      const response = await fetch("http://localhost:8080/upload", {
        method: "POST",
        body: formData,
      });

      if (response.ok) {
        console.log("File uploaded successfully");
      } else {
        console.error("Upload failed");
      }
    } catch (error) {
      console.error("Upload error:", error);
    } finally {
      setLoading(false);
    }
  };

  const getAllTorrentFiles = async () => {
    try {
      const res = await axios.get(`http://localhost:8080/active-torrents`);
      setFiles(res.data);
    } catch (error) {
      console.error("Failed to fetch the active-torrents");
    }
  };

  const getTotalDownloadedFilesInSize = async () => {
    try {
      const res = await axios.get(`http://localhost:8080/total-downloaded`);
      const totalSizeReadable = formatBytes(res.data.total_size);
      setTotalDownloaded(totalSizeReadable);
    } catch (error) {
      console.error(`Failed to get total downloaded size:`, error);
    }
  };

  // Helper function to convert bytes to GB, MB, KB, or Bytes
  function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return "0 Bytes";

    const k = 1024;
    const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return (
      parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i]
    );
  }

  const handlePause = async (fileName) => {
    try {
      await fetch(`http://localhost:8080/pause?filepath=${fileName}`, {
        method: "POST",
      });
      setFiles((prev) => ({
        ...prev,
        [fileName]: { ...files[fileName], paused: true },
      }));
    } catch (error) {
      console.error("Failed to pause torrent:", error);
    }
  };

  const handleResume = async (fileName) => {
    try {
      await fetch(`http://localhost:8080/resume?filepath=${fileName}`, {
        method: "POST",
      });
    } catch (error) {
      console.error("Failed to pause torrent:", error);
    }
  };

  useEffect(() => {
    getAllTorrentFiles();
    getTotalDownloadedFilesInSize();
  }, []);

  useEffect(() => {
    const socket = new WebSocket("ws://localhost:8080/progress");

    socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      const { torrentFile, progress } = data;
      console.log({ data });
      setFiles((prev) => ({
        ...prev,
        [progress.name]: progress,
      }));
    };

    socket.onerror = (error) => {
      console.error("WebSocket Error: ", error);
    };

    return () => {
      socket.close();
    };
  }, []);

  return (
    <AppContainer>
      <Sidebar setScreen={setScreen} inputRef={inputRef} />
      <MainContent>
        <SearchBarContainer>
          <SearchBar>
            <BiSearch className="search-icon" />
            <Input
              type="text"
              placeholder="SEARCH FOR A TORRENT"
              ref={inputRef}
              onChange={handleSearchChange}
            />
          </SearchBar>
        </SearchBarContainer>
        <TorrentContent>
          {screen === sideBar.DASHBOARD && (
            <UploadSection>
              <DownloadBox>
                <IconContainer>
                  <PiDownloadSimple style={{ fontSize: 40 }} color="black" />
                </IconContainer>
                <InfoContainer>
                  <InfoText size="large">{totalDownloaded}</InfoText>
                  <InfoText size="small">Bytes downloaded</InfoText>
                </InfoContainer>
              </DownloadBox>
              <DragDrop onFileSelect={handleFileSelect} loading={loading} />
            </UploadSection>
          )}
          <TorrentsList
            files={filteredFiles}
            handlePause={handlePause}
            handleResume={handleResume}
          />
        </TorrentContent>
      </MainContent>
    </AppContainer>
  );
}

export default App;
