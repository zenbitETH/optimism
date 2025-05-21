// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { Tournament } from "./Tournament.sol";
import { ITournamentFactory } from "interfaces/dispute/ITournamentFactory.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TournamentFactory
 * @notice Factory contract for creating Tournament instances
 */
contract TournamentFactory is ITournamentFactory, Ownable {
    /// @notice Default challenge period in seconds
    uint256 public defaultChallengePeriod = 7 days;

    /// @notice Default bond size in wei
    uint256 public defaultBondSize = 0.1 ether;

    /// @notice Mapping of tournament addresses to their validity status
    mapping(address => bool) public override isTournament;

    /// @notice Event emitted when a new tournament is created
    event TournamentCreated(
        address indexed tournament,
        bytes32 indexed rootClaim,
        uint256 indexed l2BlockNumber,
        uint256 challengePeriod,
        uint256 bondSize
    );

    /**
     * @notice Creates a new tournament for dispute resolution
     * @param rootClaim The root claim being disputed
     * @param l2BlockNumber The L2 block number associated with this dispute
     * @param extraData Extra data for tournament creation (challenge period, bond size)
     * @return The address of the newly created tournament
     */
    function createTournament(
        bytes32 rootClaim,
        uint256 l2BlockNumber,
        bytes calldata extraData
    ) external override returns (address) {
        // Decode extra data
        (uint256 challengePeriod, uint256 bondSize) = extraData.length >= 64
            ? abi.decode(extraData, (uint256, uint256))
            : (defaultChallengePeriod, defaultBondSize);

        // Create new tournament
        Tournament tournament = new Tournament(
            rootClaim,
            challengePeriod,
            bondSize,
            msg.sender,
            l2BlockNumber
        );

        address tournamentAddr = address(tournament);
        isTournament[tournamentAddr] = true;

        emit TournamentCreated(
            tournamentAddr,
            rootClaim,
            l2BlockNumber,
            challengePeriod,
            bondSize
        );

        return tournamentAddr;
    }

    /**
     * @notice Sets the default challenge period
     * @param _defaultChallengePeriod The new default challenge period in seconds
     */
    function setDefaultChallengePeriod(uint256 _defaultChallengePeriod) external onlyOwner {
        defaultChallengePeriod = _defaultChallengePeriod;
    }

    /**
     * @notice Sets the default bond size
     * @param _defaultBondSize The new default bond size in wei
     */
    function setDefaultBondSize(uint256 _defaultBondSize) external onlyOwner {
        defaultBondSize = _defaultBondSize;
    }
}