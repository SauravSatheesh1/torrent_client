import React from "react";
import styled from "@emotion/styled";
import { BsFileEarmarkArrowDown, BsFileEarmarkCheck } from "react-icons/bs";
import { AiOutlinePauseCircle } from "react-icons/ai";
import { BiCaretRightCircle } from "react-icons/bi";
import { PiClock, PiDownloadSimple } from "react-icons/pi";

// Utility function to format speed
const formatSpeed = (speedInKB) => {
  if (speedInKB >= 1024 * 1024) {
    return `${(speedInKB / (1024 * 1024)).toFixed(2)} GB/s`;
  } else if (speedInKB >= 1024) {
    return `${(speedInKB / 1024).toFixed(2)} MB/s`;
  }
  return `${speedInKB.toFixed(2)} KB/s`;
};

// Utility function to format time
const formatTime = (seconds) => {
  let years = Math.floor(seconds / (3600 * 24 * 365));
  let days = Math.floor((seconds % (3600 * 24 * 365)) / (3600 * 24));
  let hours = Math.floor((seconds % (3600 * 24)) / 3600);
  let minutes = Math.floor((seconds % 3600) / 60);
  let secs = Math.floor(seconds % 60);

  if (years > 0)
    return `${years} yr ${days} d ${hours} hr ${minutes} min ${secs} sec`;
  if (days > 0) return `${days} d ${hours} hr ${minutes} min ${secs} sec`;
  if (hours > 0) return `${hours} hr ${minutes} min ${secs} sec`;
  if (minutes > 0) return `${minutes} min ${secs} sec`;
  return `${secs} sec`;
};

const TorrentsList = ({ files, handlePause, handleResume }) => {
  return (
    <TorrentsListContainer>
      <h3>Torrents</h3>
      {Object.keys(files).map((fileName) => {
        const isPaused =
          files[fileName].paused ||
          Math.floor(files[fileName].progress) === 100;
        const isComplete = Math.floor(files[fileName].progress) === 100;

        return (
          <TorrentItem key={fileName}>
            <TorrentInfo>
              <TorrentIconContainer isComplete={isComplete}>
                {isComplete ? (
                  <BsFileEarmarkCheck style={{ fontSize: "30px" }} />
                ) : (
                  <BsFileEarmarkArrowDown style={{ fontSize: "30px" }} />
                )}
              </TorrentIconContainer>
              <TorrentDetails>
                <span>{fileName}</span>
                <TorrentProgressDetails>
                  <TorrentStats>
                    <TorrentSpeedAndTimeRemaining>
                      <TorrentSpeed>
                        {!isPaused && (
                          <>
                            <PiDownloadSimple style={{ fontSize: "14px" }} />
                            {formatSpeed(files[fileName].speed)}
                          </>
                        )}
                      </TorrentSpeed>
                      <TorrentTime>
                        {!isPaused && (
                          <>
                            <PiClock style={{ fontSize: "14px" }} />
                            {formatTime(files[fileName].remaining_time)}
                          </>
                        )}
                      </TorrentTime>
                    </TorrentSpeedAndTimeRemaining>
                    <ProgressText>
                      {Math.floor(files[fileName].progress)} %
                    </ProgressText>
                  </TorrentStats>
                  <ProgressBar>
                    <Progress
                      style={{
                        width: `${files[fileName].progress}%`,
                        backgroundColor: isComplete ? "#A6F1C9" : "#abd5e9",
                      }}
                    />
                  </ProgressBar>
                </TorrentProgressDetails>
              </TorrentDetails>
            </TorrentInfo>
            <TorrentActions
              onClick={() =>
                isPaused ? handleResume(fileName) : handlePause(fileName)
              }
            >
              {!isComplete &&
                (isPaused ? (
                  <>
                    <BiCaretRightCircle style={{ fontSize: "24px" }} />
                    Resume
                  </>
                ) : (
                  <>
                    <AiOutlinePauseCircle style={{ fontSize: "24px" }} />
                    Pause
                  </>
                ))}
            </TorrentActions>
          </TorrentItem>
        );
      })}
    </TorrentsListContainer>
  );
};

// Emotion styled components

const TorrentsListContainer = styled.div`
  width: 100%;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const TorrentItem = styled.div`
  height: 100px;
  padding: 8px;
  border-radius: 8px;
  background-color: #3a3a3a;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const TorrentInfo = styled.div`
  display: flex;
  width: 75%;
  height: 100%;
`;

const TorrentIconContainer = styled.div`
  background-color: ${(props) => (props.isComplete ? "#A6F1C9" : "#abd5e9")};
  height: 100%;
  width: 60px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const TorrentDetails = styled.div`
  display: flex;
  width: 100%;
  flex-direction: column;
  justify-content: space-between;
  padding-left: 20px;
  gap: 4px;
`;

const TorrentStats = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 10px;
`;

const TorrentSpeed = styled.div`
  display: flex;
  align-items: center;
  gap: 2px;
`;

const TorrentTime = styled.div`
  display: flex;
  align-items: center;
  gap: 2px;
`;

const TorrentSpeedAndTimeRemaining = styled.div`
  display: flex;
  gap: 16px;
`;

const TorrentProgressDetails = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const ProgressText = styled.div`
  font-size: 12px;
`;

const ProgressBar = styled.div`
  height: 4px;
  width: 100%;
  background-color: #555;
  border-radius: 20px;
  overflow: hidden;
`;

const Progress = styled.div`
  height: 4px;
`;

const TorrentActions = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-right: 10px;
  align-items: center;
  font-size: 12px;
  cursor: pointer;
`;

export default TorrentsList;
