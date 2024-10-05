import React, { useState, useRef } from "react";
import { AiFillFileAdd } from "react-icons/ai";
import styled from "@emotion/styled";

const FileUpload = ({ onFileSelect, loading }) => {
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef();

  const handleDragOver = (e) => {
    e.preventDefault();
    setDragging(true);
  };

  const handleDragLeave = () => {
    setDragging(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setDragging(false);
    const files = e.dataTransfer.files;
    if (files && files.length > 0) {
      onFileSelect(files[0]);
    }
  };

  const handleFileChange = (e) => {
    const file = e.target.files[0];
    if (file) {
      onFileSelect(file);
    }
  };

  const handleClick = () => {
    inputRef.current.click();
  };

  return (
    <FileUploadContainer
      className={dragging ? "dragging" : ""}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {loading ? (
        <Loader>Uploading...</Loader>
      ) : (
        <>
          <AiFillFileAdd />
          <Text>Drag & Drop files here or</Text>
          <BrowseButton type="button" onClick={handleClick}>
            Browse
          </BrowseButton>
          <HiddenInput
            type="file"
            ref={inputRef}
            onChange={handleFileChange}
          />
        </>
      )}
    </FileUploadContainer>
  );
};

// Emotion Styled Components

const FileUploadContainer = styled.div`
  width: 240px;
  height: 200px;
  border: 1px dashed #3a3a3a;
  background-color: #2c2c3b;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  cursor: pointer;
  transition: background-color 0.2s ease;
  padding: 10px;
  gap: 4px;

  &.dragging {
    background-color: #f0f0f0;
    border-color: #888888;
  }

  svg {
    font-size: 56px;
  }
`;

const Text = styled.p`
  font-size: 16px;
  text-align: center;
  pointer-events: none;
`;

const BrowseButton = styled.button`
  height: 40px;
  width: 100%;
  background-color: #1c1c1c;
  border: none;
  border-radius: 20px;
  color: white;
  cursor: pointer;
  font-size: 12px;
  transition: background-color 0.2s ease;

  &:hover {
    background-color: #777;
  }
`;

const Loader = styled.div`
  font-size: 20px;
  font-weight: bold;
`;

const HiddenInput = styled.input`
  display: none;
`;

export default FileUpload;
