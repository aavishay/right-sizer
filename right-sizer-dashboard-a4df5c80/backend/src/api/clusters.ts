import express, { Router, Request, Response, NextFunction } from "express";
import { body, param, query, validationResult } from "express-validator";
import validator from "validator";
import * as k8s from "@kubernetes/client-node";
import jwt, { JwtPayload, Secret, SignOptions } from "jsonwebtoken";
import { randomBytes } from "crypto";
import {
  ClusterService,
  ClusterCreateData,
  ClusterUpdateData,
} from "../services/cluster";
import { DatabaseService, db } from "../services/database";
import { CacheService } from "../services/cache";
import { authMiddleware, requireRole, operatorAuthMiddleware, apiTokenMiddleware } from "../middleware/auth";
import { validateRequest } from "../middleware/validateRequest";
import { paginationMiddleware } from "../middleware/pagination";
import { Logger } from "../middleware/requestLogger";
import { asyncHandler } from "../middleware/errorHandler";
import { UpdateClusterDto, ConnectClusterDto } from "../models/cluster";

const router: Router = express.Router();

// Initialize services
const cache = new CacheService();
const clusterService = ClusterService.getInstance(db, cache);

// Helper function for error responses
const handleError = (
  res: Response,
  error: any,
  message: string,
  statusCode: number = 500,
) => {
  Logger.error(message, error);
  const errorMessage = error instanceof Error ? error.message : message;
  res.status(statusCode).json({
    error: message,
    details: process.env.NODE_ENV === "development" ? errorMessage : undefined,
  });
};

// Validation schemas
const createClusterValidation = [
  body("name")
    .trim()
    .isLength({ min: 3, max: 63 })
    .matches(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/)
    .withMessage("Name must be a valid Kubernetes resource name"),
  body("displayName").optional().trim().isLength({ min: 1, max: 255 }),
  body("description").optional().trim().isLength({ max: 1000 }),
  body("provider").isIn([
    "aws_eks",
    "gcp_gke",
    "azure_aks",
    "openshift",
    "rancher",
    "minikube",
    "kind",
    "k3s",
    "custom",
  ]),
  body("region").optional().trim().isLength({ max: 100 }),
  body("endpoint")
    .optional()
    .trim()
    .custom((value) => {
      if (value && value.length > 0) {
        if (!validator.isURL(value)) {
          throw new Error("Endpoint must be a valid URL");
        }
      }
      return true;
    }),
  body("version")
    .optional()
    .trim()
    .matches(/^v?\d+\.\d+(\.\d+)?$/),
  body("kubeconfig").optional().isString(),
  // Removed apiTokenId validation - handled in helm-values endpoint
  // body("apiTokenId")
  //   .optional({ checkFalsy: true })
  //   .isUUID(),
  body("tags").optional().isObject(),
  body("config").optional().isObject(),
];

const updateClusterValidation = [
  param("clusterId").isUUID(),
  body("displayName").optional().trim().isLength({ min: 1, max: 255 }),
  body("description").optional().trim().isLength({ max: 1000 }),
  body("provider")
    .optional()
    .isIn([
      "aws_eks",
      "gcp_gke",
      "azure_aks",
      "openshift",
      "rancher",
      "minikube",
      "kind",
      "k3s",
      "custom",
    ]),
  body("region").optional().trim().isLength({ max: 100 }),
  body("endpoint").optional().trim().isURL(),
  body("version")
    .optional()
    .trim()
    .matches(/^v?\d+\.\d+(\.\d+)?$/),
  body("kubeconfig").optional().isString(),
  body("status").optional().isIn(["connected", "disconnected", "error"]),
  body("tags").optional().isObject(),
  body("config").optional().isObject(),
];

const connectClusterValidation = [
  body("kubeconfig").isString().withMessage("Kubeconfig is required"),
];

// Get all clusters
router.get(
  "/",
  authMiddleware,
  query("status").optional().isIn(["connected", "disconnected", "error"]),
  query("provider")
    .optional()
    .isIn([
      "aws_eks",
      "gcp_gke",
      "azure_aks",
      "openshift",
      "rancher",
      "minikube",
      "kind",
      "k3s",
      "custom",
    ]),
  query("region").optional().isString(),
  asyncHandler(async (req: Request, res: Response) => {
    try {
      const organizationId =
        (req as any).user.organizationId || (req as any).user.id;
      const { status, provider, region } = req.query;

      // Build filter conditions
      const filters: any = { created_by: organizationId };
      if (status) filters.status = status;
      if (provider) filters.provider = provider;
      if (region) filters.region = region;

      // Get clusters from service
      const clusters =
        await clusterService.getClustersByOrganization(organizationId);

      // Apply additional filters that aren't in the database query
      const filteredClusters = clusters.filter((cluster) => {
        if (provider && cluster.provider !== provider) return false;
        if (region && cluster.region !== region) return false;
        return true;
      });

      // Get connection status for each cluster
      const clustersWithStatus = await Promise.all(
        filteredClusters.map(async (cluster) => {
          const connection = await clusterService.getClusterHealth(cluster.id);
          return {
            ...cluster,
            connection_status: connection?.status || "unknown",
            last_heartbeat: connection?.last_heartbeat,
          };
        }),
      );

      res.json({
        clusters: clustersWithStatus,
        count: clustersWithStatus.length,
      });
    } catch (error) {
      handleError(res, error, "Failed to fetch clusters");
    }
  }),
);

// POST /api/clusters/status - Receive cluster status updates from operator
router.post(
  "/status",
  operatorAuthMiddleware,
  asyncHandler(async (req: Request, res: Response) => {
    try {
      console.log("📊 [/status handler] Reached cluster status POST handler");
      const { clusterId, clusterName, status, lastSeen, metadata } = req.body;
      console.log(`📊 [/status handler] Body: clusterId=${clusterId}, clusterName=${clusterName}, status=${status}`);

      // Validate required fields
      if (!status) {
        console.log("📊 [/status handler] Missing required field: status");
        return res.status(400).json({
          error: "Missing required field: status",
        });
      }

      if (!clusterId && !clusterName) {
        console.log("📊 [/status handler] Missing both clusterId and clusterName");
        return res.status(400).json({
          error: "Either clusterId or clusterName must be provided",
        });
      }

      // Store status in cache using clusterName as key if no ID
      const cacheKey = clusterId
        ? `cluster:health:${clusterId}`
        : `cluster:health:name:${clusterName}`;

      const healthUpdate = {
        clusterId: clusterId || "unknown",
        clusterName: clusterName || "unknown",
        status: status === "connected" ? "connected" : "disconnected",
        connectivity: status === "connected" ? "online" : "offline",
        lastCheck: lastSeen || new Date().toISOString(),
        uptime: (metadata as any)?.uptime || 0,
        message: (metadata as any)?.message || `Status: ${status}`,
        receivedAt: new Date().toISOString(),
      };

      console.log("📊 [/status handler] Caching status update...");
      // Cache the health status for 5 minutes
      await cache.set(cacheKey, healthUpdate, 300);
      console.log("📊 [/status handler] Status cached successfully");

      res.json({
        message: "Cluster status received",
        clusterId: clusterId || null,
        clusterName: clusterName || null,
        status: healthUpdate.status,
      });
    } catch (error) {
      handleError(res, error, "Failed to process cluster status update");
    }
  }),
);

export default router;
