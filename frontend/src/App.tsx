import { useState } from "react";
import { Layout } from "@/components/Layout";
import { Lights } from "@/components/Lights";
import { Scenes } from "@/components/Scenes";
import { Settings } from "@/components/Settings";

function App() {
  const [currentPage, setCurrentPage] = useState("lights");

  const renderPage = () => {
    switch (currentPage) {
      case "lights":
        return <Lights />;
      case "scenes":
        return <Scenes />;
      case "settings":
        return <Settings />;
      default:
        return <Lights />;
    }
  };

  return (
    <Layout currentPage={currentPage} onNavigate={setCurrentPage}>
      {renderPage()}
    </Layout>
  );
}

export default App;
